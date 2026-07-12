#!/usr/bin/env python3
"""
Local NetGoat demo lab.

This script is intentionally local-only by default. It serves:
  - an admin/demo UI on 127.0.0.1:8890
  - an upstream target app on 127.0.0.1:8888

The UI can seed local route/auth data, edit SQLite WAF rules, toggle
zero-trust settings, and fire requests through a locally running NetGoat agent.
"""

from __future__ import annotations

import argparse
import json
import sqlite3
import threading
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


DEFAULT_DB = "database/proxy.db"
DEFAULT_DOMAIN = "app.netgoat.test"
DEFAULT_AGENT_PORT = 8080
DEFAULT_UI_PORT = 8890
DEFAULT_UPSTREAM_PORT = 8888
DEFAULT_BIND = "127.0.0.1"
DEFAULT_ADMIN_HASH = "$2a$10$qxhmdluck7osE8nR1bwcZeUloJOnFksfPoWvUT2wkDKQIhvoSWXha"


DEFAULT_WAF_RULES = [
    ("Block Admin", 'Path startsWith "/admin"', "BLOCK", 10),
    ("Block SQL Injection (Path)", r'Path matches ".*(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;).*$"', "BLOCK", 20),
    ("Block SQL Injection (Query)", r'RawQuery matches "(?i)(union\\s+select|waitfor\\s+delay|1=1|--|;)"', "BLOCK", 20),
    ("Block XSS (Path)", r'Path matches "(?i)(<script>|javascript:|onerror=)"', "BLOCK", 20),
    ("Block XSS (Query)", r'RawQuery matches "(?i)(<script>|javascript:|onerror=)"', "BLOCK", 20),
    ("Block Path Traversal", r'Path matches "(?:\\.\\./|\\.\\.\\\\)"', "BLOCK", 20),
    ("Block SSRF Metadata & Localhost", r'RawQuery matches "(?i)(169\\.254\\.169\\.254|127\\.0\\.0\\.1|localhost)"', "BLOCK", 20),
]


def connect_db(path: str) -> sqlite3.Connection:
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    ensure_schema(conn)
    return conn


def ensure_schema(conn: sqlite3.Connection) -> None:
    conn.executescript(
        """
        CREATE TABLE IF NOT EXISTS routes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            route_type TEXT NOT NULL DEFAULT 'domain',
            domain TEXT,
            path_prefix TEXT,
            target_url TEXT NOT NULL,
            certificate_pem TEXT,
            private_key_pem TEXT,
            active INTEGER DEFAULT 1,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(route_type, domain, path_prefix)
        );

        CREATE TABLE IF NOT EXISTS route_targets (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            route_id INTEGER NOT NULL,
            target_url TEXT NOT NULL,
            health_check TEXT NOT NULL DEFAULT 'http',
            sort_order INTEGER NOT NULL DEFAULT 0,
            FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE,
            UNIQUE(route_id, target_url)
        );

        CREATE TABLE IF NOT EXISTS waf_rules (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            expression TEXT NOT NULL,
            action TEXT NOT NULL DEFAULT 'BLOCK',
            priority INTEGER DEFAULT 0,
            UNIQUE(name)
        );

        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            email TEXT,
            zero_trust_enabled INTEGER DEFAULT 1,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS user_sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            token TEXT NOT NULL UNIQUE,
            expires_at DATETIME NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id)
        );

        CREATE TABLE IF NOT EXISTS zero_trust_settings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            key TEXT NOT NULL UNIQUE,
            value TEXT NOT NULL,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );
        """
    )
    conn.commit()


def seed_route(db_path: str, domain: str, target: str) -> None:
    with connect_db(db_path) as conn:
        conn.execute(
            """
            INSERT INTO routes (route_type, domain, path_prefix, target_url, active)
            VALUES ('domain', ?, '', ?, 1)
            ON CONFLICT(route_type, domain, path_prefix)
            DO UPDATE SET target_url=excluded.target_url, active=1, updated_at=CURRENT_TIMESTAMP
            """,
            (domain, target),
        )
        route_id = conn.execute(
            "SELECT id FROM routes WHERE route_type='domain' AND domain=? AND path_prefix=''",
            (domain,),
        ).fetchone()["id"]
        conn.execute("DELETE FROM route_targets WHERE route_id=?", (route_id,))
        conn.execute(
            "INSERT INTO route_targets (route_id, target_url, health_check, sort_order) VALUES (?, ?, 'http', 0)",
            (route_id, target),
        )
        conn.commit()


def seed_auth(db_path: str) -> None:
    with connect_db(db_path) as conn:
        conn.execute(
            """
            INSERT INTO users (username, password_hash, email, zero_trust_enabled)
            VALUES ('admin', ?, 'admin@local.test', 1)
            ON CONFLICT(username) DO UPDATE SET
                password_hash=excluded.password_hash,
                email=excluded.email
            """,
            (DEFAULT_ADMIN_HASH,),
        )
        conn.execute(
            """
            INSERT INTO zero_trust_settings (key, value)
            VALUES ('enabled', 'true')
            ON CONFLICT(key) DO NOTHING
            """
        )
        conn.commit()


def reset_waf(db_path: str) -> None:
    with connect_db(db_path) as conn:
        conn.execute("DELETE FROM waf_rules")
        conn.executemany(
            "INSERT INTO waf_rules (name, expression, action, priority) VALUES (?, ?, ?, ?)",
            DEFAULT_WAF_RULES,
        )
        conn.commit()


def list_state(db_path: str) -> dict:
    with connect_db(db_path) as conn:
        routes = [
            dict(row)
            for row in conn.execute(
                """
                SELECT id, route_type, domain, path_prefix, target_url, active
                FROM routes
                ORDER BY id ASC
                """
            ).fetchall()
        ]
        rules = [
            dict(row)
            for row in conn.execute(
                """
                SELECT id, name, expression, action, priority
                FROM waf_rules
                ORDER BY priority DESC, name ASC
                """
            ).fetchall()
        ]
        users = [
            dict(row)
            for row in conn.execute(
                """
                SELECT id, username, email, zero_trust_enabled
                FROM users
                ORDER BY username ASC
                """
            ).fetchall()
        ]
        global_zero_trust = conn.execute(
            "SELECT value FROM zero_trust_settings WHERE key='enabled'"
        ).fetchone()
    return {
        "routes": routes,
        "waf_rules": rules,
        "users": users,
        "zero_trust_enabled": (global_zero_trust["value"] if global_zero_trust else "true") == "true",
    }


def set_zero_trust(db_path: str, payload: dict) -> None:
    global_enabled = bool(payload.get("global_enabled", True))
    username = str(payload.get("username", "admin")).strip() or "admin"
    user_enabled = 1 if bool(payload.get("user_enabled", True)) else 0
    with connect_db(db_path) as conn:
        conn.execute(
            """
            INSERT INTO zero_trust_settings (key, value)
            VALUES ('enabled', ?)
            ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
            """,
            ("true" if global_enabled else "false",),
        )
        result = conn.execute(
            "UPDATE users SET zero_trust_enabled=? WHERE username=?",
            (user_enabled, username),
        )
        if result.rowcount == 0:
            raise ValueError(f"user {username!r} does not exist; seed auth first")
        conn.commit()


def upsert_waf_rule(db_path: str, payload: dict) -> None:
    name = str(payload.get("name", "")).strip()
    expression = str(payload.get("expression", "")).strip()
    action = str(payload.get("action", "BLOCK")).strip().upper() or "BLOCK"
    priority = int(payload.get("priority", 50))

    if not name:
        raise ValueError("name is required")
    if not expression:
        raise ValueError("expression is required")
    if action != "BLOCK":
        raise ValueError("only BLOCK action is supported by the current agent WAF")

    with connect_db(db_path) as conn:
        conn.execute(
            """
            INSERT INTO waf_rules (name, expression, action, priority)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(name)
            DO UPDATE SET expression=excluded.expression, action=excluded.action, priority=excluded.priority
            """,
            (name, expression, action, priority),
        )
        conn.commit()


def delete_waf_rule(db_path: str, name: str) -> None:
    with connect_db(db_path) as conn:
        conn.execute("DELETE FROM waf_rules WHERE name=?", (name,))
        conn.commit()


class DemoState:
    def __init__(self, args: argparse.Namespace):
        self.db_path = args.db
        self.domain = args.domain
        self.agent_port = args.agent_port
        self.ui_port = args.ui_port
        self.upstream_port = args.upstream_port
        self.bind = args.bind

    @property
    def upstream_url(self) -> str:
        return f"http://127.0.0.1:{self.upstream_port}"

    @property
    def proxy_url(self) -> str:
        return f"http://{self.domain}:{self.agent_port}"


def read_json(handler: BaseHTTPRequestHandler) -> dict:
    length = int(handler.headers.get("Content-Length", "0"))
    if length > 64 * 1024:
        raise ValueError("request body too large")
    raw = handler.rfile.read(length) if length else b"{}"
    return json.loads(raw.decode("utf-8") or "{}")


def send_json(handler: BaseHTTPRequestHandler, status: int, payload: dict) -> None:
    body = json.dumps(payload, indent=2).encode("utf-8")
    handler.send_response(status)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.send_header("Access-Control-Allow-Origin", "*")
    handler.end_headers()
    handler.wfile.write(body)


def make_admin_handler(state: DemoState):
    class AdminHandler(BaseHTTPRequestHandler):
        server_version = "NetGoatDemoLab/1.0"

        def do_OPTIONS(self) -> None:
            self.send_response(HTTPStatus.NO_CONTENT)
            self.send_header("Access-Control-Allow-Origin", "*")
            self.send_header("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type")
            self.end_headers()

        def do_GET(self) -> None:
            parsed = urlparse(self.path)
            if parsed.path == "/":
                self.send_html(render_demo_html(state))
                return
            if parsed.path == "/api/state":
                send_json(self, 200, list_state(state.db_path))
                return
            if parsed.path == "/api/info":
                send_json(
                    self,
                    200,
                    {
                        "domain": state.domain,
                        "proxy_url": state.proxy_url,
                        "upstream_url": state.upstream_url,
                        "db_path": state.db_path,
                    },
                )
                return
            self.send_error(HTTPStatus.NOT_FOUND)

        def do_POST(self) -> None:
            parsed = urlparse(self.path)
            try:
                if parsed.path == "/api/seed":
                    seed_route(state.db_path, state.domain, state.upstream_url)
                    seed_auth(state.db_path)
                    send_json(self, 200, {"ok": True})
                    return
                if parsed.path == "/api/reset-waf":
                    reset_waf(state.db_path)
                    send_json(self, 200, {"ok": True})
                    return
                if parsed.path == "/api/zero-trust":
                    set_zero_trust(state.db_path, read_json(self))
                    send_json(self, 200, {"ok": True})
                    return
                if parsed.path == "/api/waf-rule":
                    upsert_waf_rule(state.db_path, read_json(self))
                    send_json(self, 200, {"ok": True})
                    return
                self.send_error(HTTPStatus.NOT_FOUND)
            except Exception as exc:
                send_json(self, 400, {"ok": False, "error": str(exc)})

        def do_DELETE(self) -> None:
            parsed = urlparse(self.path)
            if parsed.path != "/api/waf-rule":
                self.send_error(HTTPStatus.NOT_FOUND)
                return
            name = parse_qs(parsed.query).get("name", [""])[0]
            if not name:
                send_json(self, 400, {"ok": False, "error": "name is required"})
                return
            delete_waf_rule(state.db_path, name)
            send_json(self, 200, {"ok": True})

        def send_html(self, html: str) -> None:
            body = html.encode("utf-8")
            self.send_response(HTTPStatus.OK)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, fmt: str, *args) -> None:
            print(f"[demo-ui] {self.address_string()} {fmt % args}")

    return AdminHandler


def make_upstream_handler():
    class UpstreamHandler(BaseHTTPRequestHandler):
        server_version = "NetGoatDemoUpstream/1.0"

        def do_OPTIONS(self) -> None:
            self.send_response(HTTPStatus.NO_CONTENT)
            self.cors()
            self.send_header("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
            self.send_header(
                "Access-Control-Allow-Headers",
                "Content-Type, X-GoatAI-Features, X-KodaWaf-Features, X-Koda2-Features",
            )
            self.end_headers()

        def do_GET(self) -> None:
            parsed = urlparse(self.path)
            if parsed.path == "/large":
                size = int(parse_qs(parsed.query).get("bytes", ["262144"])[0])
                body = (b"NetGoat bandwidth test\n" * ((size // 23) + 1))[:size]
                self.send_response(HTTPStatus.OK)
                self.cors()
                self.send_header("Content-Type", "application/octet-stream")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return

            if parsed.path == "/private-cache":
                body = json.dumps({"kind": "private", "time": time.time()}).encode("utf-8")
                self.send_response(HTTPStatus.OK)
                self.cors()
                self.send_header("Content-Type", "application/json")
                self.send_header("Cache-Control", "private, max-age=60")
                self.send_header("Set-Cookie", "demo_session=local-only; Path=/; HttpOnly")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return

            if parsed.path == "/public-cache":
                body = json.dumps({"kind": "public", "time": time.time()}).encode("utf-8")
                self.send_response(HTTPStatus.OK)
                self.cors()
                self.send_header("Content-Type", "application/json")
                self.send_header("Cache-Control", "public, max-age=60")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return

            if parsed.path == "/api/echo":
                body = json.dumps(
                    {
                        "path": parsed.path,
                        "query": parse_qs(parsed.query),
                        "headers": {
                            k: v
                            for k, v in self.headers.items()
                            if k.lower().startswith("x-") or k.lower() in {"host", "user-agent"}
                        },
                        "time": time.time(),
                    },
                    indent=2,
                ).encode("utf-8")
                self.send_response(HTTPStatus.OK)
                self.cors()
                self.send_header("Content-Type", "application/json")
                self.send_header("Cache-Control", "no-store")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return

            body = render_upstream_html().encode("utf-8")
            self.send_response(HTTPStatus.OK)
            self.cors()
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Cache-Control", "public, max-age=30")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def cors(self) -> None:
            self.send_header("Access-Control-Allow-Origin", "*")

        def log_message(self, fmt: str, *args) -> None:
            print(f"[upstream] {self.address_string()} {fmt % args}")

    return UpstreamHandler


def render_upstream_html() -> str:
    return """<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NetGoat Local Upstream</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 0; padding: 48px; background: #f7f8fa; color: #111827; }
    main { max-width: 760px; margin: 0 auto; background: white; border: 1px solid #d7dce2; padding: 28px; border-radius: 8px; }
    code { background: #eef1f5; padding: 2px 6px; border-radius: 4px; }
  </style>
</head>
<body>
  <main>
    <h1>NetGoat local upstream</h1>
    <p>This page is served by the local demo upstream and should be reached through the NetGoat agent.</p>
    <p>Try <code>/admin</code>, <code>/?q=union select</code>, <code>/public-cache</code>, <code>/private-cache</code>, and <code>/large?bytes=262144</code>.</p>
  </main>
</body>
</html>"""


def render_demo_html(state: DemoState) -> str:
    config_snippet = f"""routes:
  {state.domain}:
    type: "domain"
    targets:
      - url: "{state.upstream_url}"
        health_check: "http"
"""
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>NetGoat Local Demo Lab</title>
  <style>
    :root {{
      color-scheme: light;
      --bg: #f5f6f8;
      --panel: #ffffff;
      --line: #d8dee6;
      --text: #14171f;
      --muted: #667085;
      --accent: #2f6f73;
      --danger: #a13f3f;
      --warn: #9a6a22;
      --ok: #237a4d;
    }}
    * {{ box-sizing: border-box; }}
    body {{ margin: 0; background: var(--bg); color: var(--text); font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }}
    header {{ border-bottom: 1px solid var(--line); background: #fff; }}
    .wrap {{ max-width: 1320px; margin: 0 auto; padding: 20px; }}
    .top {{ display: grid; grid-template-columns: 1fr auto; gap: 16px; align-items: end; }}
    h1 {{ margin: 0 0 6px; font-size: 28px; letter-spacing: 0; }}
    p {{ margin: 0; color: var(--muted); }}
    code, pre, textarea, input {{ font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }}
    button, input, textarea, select {{ font: inherit; }}
    button {{ border: 1px solid var(--line); background: #fff; color: var(--text); border-radius: 6px; padding: 8px 11px; cursor: pointer; }}
    button:hover {{ border-color: var(--accent); }}
    button.primary {{ background: var(--accent); border-color: var(--accent); color: #fff; }}
    button.danger {{ color: var(--danger); }}
    main.wrap {{ display: grid; grid-template-columns: 360px 1fr; gap: 18px; }}
    section {{ background: var(--panel); border: 1px solid var(--line); border-radius: 8px; }}
    .section-head {{ padding: 14px 16px; border-bottom: 1px solid var(--line); display: flex; justify-content: space-between; gap: 12px; align-items: center; }}
    .section-head h2 {{ margin: 0; font-size: 16px; }}
    .section-body {{ padding: 16px; }}
    .stack {{ display: grid; gap: 12px; }}
    .row {{ display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }}
    label {{ display: grid; gap: 5px; color: var(--muted); font-size: 12px; }}
    input, textarea, select {{ width: 100%; border: 1px solid var(--line); border-radius: 6px; padding: 8px 9px; background: #fff; color: var(--text); }}
    textarea {{ min-height: 82px; resize: vertical; }}
    pre {{ margin: 0; white-space: pre-wrap; background: #101828; color: #edf2f7; padding: 12px; border-radius: 6px; overflow: auto; max-height: 260px; }}
    .grid {{ display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }}
    .rule {{ display: grid; gap: 8px; padding: 12px; border: 1px solid var(--line); border-radius: 6px; }}
    .rule strong {{ font-size: 13px; }}
    .rule code {{ display: block; color: #344054; overflow-wrap: anywhere; }}
    .pill {{ display: inline-flex; align-items: center; border: 1px solid var(--line); border-radius: 999px; padding: 3px 8px; font-size: 12px; color: var(--muted); background: #fff; }}
    .log {{ display: grid; gap: 8px; max-height: 440px; overflow: auto; }}
    .entry {{ border-left: 3px solid var(--line); background: #fafbfc; padding: 10px; border-radius: 4px; }}
    .entry.ok {{ border-left-color: var(--ok); }}
    .entry.block {{ border-left-color: var(--danger); }}
    .entry.warn {{ border-left-color: var(--warn); }}
    .entry-title {{ display: flex; justify-content: space-between; gap: 12px; margin-bottom: 6px; }}
    .muted {{ color: var(--muted); }}
    .mono {{ font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }}
    @media (max-width: 980px) {{ main.wrap, .grid, .top {{ grid-template-columns: 1fr; }} }}
  </style>
</head>
<body>
  <header>
    <div class="wrap top">
      <div>
        <h1>NetGoat Local Demo Lab</h1>
        <p>Local-only WAF and proxy feature testing. UI: <span class="mono">127.0.0.1:{state.ui_port}</span>. Upstream: <span class="mono">{state.upstream_url}</span>.</p>
      </div>
      <div class="row">
        <span class="pill">Domain: {state.domain}</span>
        <span class="pill">Agent: {state.proxy_url}</span>
      </div>
    </div>
  </header>

  <main class="wrap">
    <aside class="stack">
      <section>
        <div class="section-head"><h2>Setup</h2></div>
        <div class="section-body stack">
          <p>Run this once if you have not added the fake domain yet:</p>
          <pre>sudo scripts/dev-fake-domain.sh add --domain {state.domain}</pre>
          <button class="primary" onclick="seedRoute()">Seed route and auth</button>
          <button onclick="resetWaf()">Reset default WAF rules</button>
          <pre>{config_snippet}</pre>
        </div>
      </section>

      <section>
        <div class="section-head"><h2>Zero-Trust</h2></div>
        <div class="section-body stack">
          <p>Default local login is <span class="mono">admin</span> / <span class="mono">admin</span>.</p>
          <label>Username
            <input id="ztUser" value="admin">
          </label>
          <label><span><input id="ztGlobal" type="checkbox" checked style="width:auto"> Global zero-trust enabled</span></label>
          <label><span><input id="ztUserEnabled" type="checkbox" checked style="width:auto"> User requires zero-trust</span></label>
          <button class="primary" onclick="saveZeroTrust()">Save zero-trust</button>
          <div id="users" class="stack"></div>
        </div>
      </section>

      <section>
        <div class="section-head"><h2>WAF Rule Editor</h2></div>
        <div class="section-body stack">
          <label>Name
            <input id="ruleName" value="Block Demo Header">
          </label>
          <label>Expression
            <textarea id="ruleExpression">Headers["X-Demo-Block"][0] == "1"</textarea>
          </label>
          <div class="row">
            <label style="flex: 1">Priority
              <input id="rulePriority" type="number" value="80">
            </label>
            <label style="flex: 1">Action
              <select id="ruleAction"><option>BLOCK</option></select>
            </label>
          </div>
          <button class="primary" onclick="saveRule()">Save rule</button>
        </div>
      </section>
    </aside>

    <div class="stack">
      <section>
        <div class="section-head">
          <h2>Feature Requests</h2>
          <button onclick="clearLog()">Clear log</button>
        </div>
        <div class="section-body grid">
          <button onclick="openProxy('/')">Open proxied page</button>
          <button onclick="sendScenario('Default WAF /admin', '/admin')">WAF path block</button>
          <button onclick="sendScenario('SQL query block', '/?q=union%20select')">WAF SQL query</button>
          <button onclick="sendScenario('SSRF query block', '/?next=http://169.254.169.254/latest')">WAF SSRF query</button>
          <button onclick="sendScenario('Custom header rule', '/api/echo', {{'X-Demo-Block': '1'}})">Custom header WAF</button>
          <button onclick="cacheScenario()">Cache public twice</button>
          <button onclick="sendScenario('Private cache rejected', '/private-cache')">Private cache safety</button>
          <button onclick="rateScenario()">Rate limit burst</button>
          <button onclick="sendScenario('Bandwidth download', '/large?bytes=262144')">Bandwidth download</button>
          <button onclick="openProxy('/login')">Zero-trust login</button>
          <button onclick="sendScenario('GoatAI header', '/api/echo?goatai=1', {{'X-GoatAI-Features': '0.9,0.95,0.88,0.92,100,999'}})">GoatAI header</button>
          <button onclick="sendScenario('Koda WAF header', '/api/echo?koda=1', {{'X-KodaWaf-Features': '1,2,3,4,5,6'}})">Koda WAF header</button>
          <button onclick="sendScenario('Koda-2 header', '/api/echo?koda2=1', {{'X-Koda2-Features': '1,2,3,4,5,6'}})">Koda-2 header</button>
        </div>
      </section>

      <section>
        <div class="section-head">
          <h2>Local WAF Rules</h2>
          <button onclick="refreshState()">Refresh</button>
        </div>
        <div class="section-body">
          <div id="rules" class="stack"></div>
        </div>
      </section>

      <section>
        <div class="section-head"><h2>Request Log</h2></div>
        <div class="section-body">
          <div id="log" class="log"></div>
        </div>
      </section>
    </div>
  </main>

<script>
const proxyBase = "{state.proxy_url}";

function $(id) {{ return document.getElementById(id); }}

async function api(path, options = {{}}) {{
  const res = await fetch(path, {{
    headers: {{'Content-Type': 'application/json'}},
    ...options
  }});
  const data = await res.json();
  if (!res.ok || data.ok === false) throw new Error(data.error || res.statusText);
  return data;
}}

function addLog(title, detail, kind = 'ok') {{
  const el = document.createElement('div');
  el.className = `entry ${{kind}}`;
  el.innerHTML = `<div class="entry-title"><strong>${{title}}</strong><span class="muted">${{new Date().toLocaleTimeString()}}</span></div><pre>${{escapeHtml(detail)}}</pre>`;
  $('log').prepend(el);
}}

function escapeHtml(value) {{
  return String(value).replace(/[&<>"']/g, ch => ({{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}}[ch]));
}}

async function refreshState() {{
  const state = await api('/api/state');
  $('rules').innerHTML = '';
  $('users').innerHTML = '';
  $('ztGlobal').checked = Boolean(state.zero_trust_enabled);
  for (const rule of state.waf_rules) {{
    const el = document.createElement('div');
    el.className = 'rule';
    el.innerHTML = `
      <div class="entry-title">
        <strong>${{escapeHtml(rule.name)}}</strong>
        <span class="pill">priority ${{rule.priority}}</span>
      </div>
      <code>${{escapeHtml(rule.expression)}}</code>
      <div class="row">
        <button onclick='loadRule(${{JSON.stringify(rule)}})'>Edit</button>
        <button class="danger" onclick='deleteRule(${{JSON.stringify(rule.name)}})'>Delete</button>
      </div>
    `;
    $('rules').appendChild(el);
  }}
  for (const user of state.users) {{
    const el = document.createElement('div');
    el.className = 'rule';
    el.innerHTML = `
      <div class="entry-title">
        <strong>${{escapeHtml(user.username)}}</strong>
        <span class="pill">${{user.zero_trust_enabled ? 'zero-trust on' : 'zero-trust off'}}</span>
      </div>
      <div class="row">
        <button onclick='loadZeroTrustUser(${{JSON.stringify(user)}})'>Load user</button>
      </div>
    `;
    $('users').appendChild(el);
  }}
}}

function loadRule(rule) {{
  $('ruleName').value = rule.name;
  $('ruleExpression').value = rule.expression;
  $('rulePriority').value = rule.priority;
  $('ruleAction').value = rule.action;
}}

async function seedRoute() {{
  await api('/api/seed', {{method: 'POST', body: '{{}}'}});
  addLog('Seed route and auth', 'Route points {state.domain} to {state.upstream_url}\\nLogin: admin / admin');
  await refreshState();
}}

async function resetWaf() {{
  await api('/api/reset-waf', {{method: 'POST', body: '{{}}'}});
  addLog('Reset WAF', 'Default WAF rules restored.');
  await refreshState();
}}

async function saveRule() {{
  const payload = {{
    name: $('ruleName').value,
    expression: $('ruleExpression').value,
    action: $('ruleAction').value,
    priority: Number($('rulePriority').value || 50)
  }};
  await api('/api/waf-rule', {{method: 'POST', body: JSON.stringify(payload)}});
  addLog('Saved WAF rule', JSON.stringify(payload, null, 2));
  await refreshState();
}}

async function deleteRule(name) {{
  await api('/api/waf-rule?name=' + encodeURIComponent(name), {{method: 'DELETE'}});
  addLog('Deleted WAF rule', name, 'warn');
  await refreshState();
}}

function loadZeroTrustUser(user) {{
  $('ztUser').value = user.username;
  $('ztUserEnabled').checked = Boolean(user.zero_trust_enabled);
}}

async function saveZeroTrust() {{
  const payload = {{
    username: $('ztUser').value,
    global_enabled: $('ztGlobal').checked,
    user_enabled: $('ztUserEnabled').checked
  }};
  await api('/api/zero-trust', {{method: 'POST', body: JSON.stringify(payload)}});
  addLog('Saved zero-trust', JSON.stringify(payload, null, 2));
  await refreshState();
}}

async function sendScenario(title, path, headers = {{}}) {{
  const started = performance.now();
  try {{
    const res = await fetch(proxyBase + path, {{headers}});
    const text = await res.text();
    const ms = Math.round(performance.now() - started);
    const detail = [
      `URL: ${{proxyBase + path}}`,
      `Status: ${{res.status}}`,
      `X-Cache: ${{res.headers.get('X-Cache') || '(none)'}}`,
      `Duration: ${{ms}}ms`,
      '',
      text.slice(0, 900)
    ].join('\\n');
    addLog(title, detail, res.status >= 400 ? 'block' : 'ok');
  }} catch (error) {{
    addLog(title, error.message, 'block');
  }}
}}

async function cacheScenario() {{
  await sendScenario('Cache public first request', '/public-cache');
  await sendScenario('Cache public second request', '/public-cache');
}}

async function rateScenario() {{
  const requests = [];
  for (let i = 0; i < 20; i++) {{
    requests.push(fetch(proxyBase + '/api/echo?burst=' + i).then(res => res.status).catch(() => 'ERR'));
  }}
  const statuses = await Promise.all(requests);
  addLog('Rate limit burst', statuses.join(', '), statuses.some(s => s === 429) ? 'warn' : 'ok');
}}

function openProxy(path) {{
  window.open(proxyBase + path, '_blank', 'noopener,noreferrer');
}}

function clearLog() {{
  $('log').innerHTML = '';
}}

refreshState().catch(error => addLog('State load failed', error.message, 'block'));
</script>
</body>
</html>"""


def serve(name: str, bind: str, port: int, handler) -> ThreadingHTTPServer:
    server = ThreadingHTTPServer((bind, port), handler)
    thread = threading.Thread(target=server.serve_forever, name=name, daemon=True)
    thread.start()
    return server


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run a local-only NetGoat demo lab.")
    parser.add_argument("--db", default=DEFAULT_DB, help=f"SQLite DB path. Default: {DEFAULT_DB}")
    parser.add_argument("--domain", default=DEFAULT_DOMAIN, help=f"Fake domain routed through NetGoat. Default: {DEFAULT_DOMAIN}")
    parser.add_argument("--agent-port", type=int, default=DEFAULT_AGENT_PORT, help="Local NetGoat agent port.")
    parser.add_argument("--ui-port", type=int, default=DEFAULT_UI_PORT, help="Local demo UI port.")
    parser.add_argument("--upstream-port", type=int, default=DEFAULT_UPSTREAM_PORT, help="Local upstream app port.")
    parser.add_argument("--bind", default=DEFAULT_BIND, help="Bind address. Defaults to 127.0.0.1 for local-only use.")
    parser.add_argument("--seed", action="store_true", help="Seed route and default WAF rules at startup.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    if args.bind not in {"127.0.0.1", "localhost", "::1"}:
        raise SystemExit("Refusing non-loopback bind address. This lab is intended for local-only use.")

    state = DemoState(args)
    ensure_schema(connect_db(state.db_path))
    if args.seed:
        seed_route(state.db_path, state.domain, state.upstream_url)
        seed_auth(state.db_path)
        reset_waf(state.db_path)

    upstream = serve("netgoat-demo-upstream", state.bind, state.upstream_port, make_upstream_handler())
    admin = serve("netgoat-demo-ui", state.bind, state.ui_port, make_admin_handler(state))

    print("NetGoat local demo lab running")
    print(f"  UI:       http://{state.bind}:{state.ui_port}")
    print(f"  Upstream: {state.upstream_url}")
    print(f"  Proxy:    {state.proxy_url}")
    print()
    print("If the fake domain is not set yet, run:")
    print(f"  sudo scripts/dev-fake-domain.sh add --domain {state.domain}")
    print()
    print("Start the NetGoat agent in another terminal:")
    print("  ./agent")
    print()
    print("Press Ctrl-C to stop the demo lab.")

    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        print("\nStopping demo lab...")
        admin.shutdown()
        upstream.shutdown()


if __name__ == "__main__":
    main()
