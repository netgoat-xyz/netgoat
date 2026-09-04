# Operations guide

## Route-scoped cache and bandwidth policy

Global `cache` and `bandwidth` settings remain the defaults. A route can
override individual fields beneath `routes.<route>.policy`; omitted fields
inherit the global setting, while `enabled: false` disables that facility for
the route. Each route uses an isolated response cache and bandwidth namespace,
so activity on one route cannot evict another route's cache entries or consume
its client quota.

```yaml
routes:
  api.example.test:
    type: domain
    targets:
      - url: http://127.0.0.1:3000
    policy:
      cache:
        enabled: true
        ttl_seconds: 30
        max_entries: 500
      bandwidth:
        enabled: true
        bytes_per_second: 1048576
        burst_bytes: 2097152
        key: ip
```

The cache still only stores responses that are safe for shared caching. It does
not override upstream `Cache-Control`, authentication, cookie, or response
freshness safeguards.

## TLS and ACME

Plaintext HTTP is allowed only on loopback (`127.0.0.1`, `::1`, `localhost`)
or when `allow_insecure_public_http: true` is set. A public bind without TLS
is refused at startup. Set `listen` to the desired HTTP address; an omitted
value still means `:8080` and therefore requires the insecure flag or TLS.

When `ssl.enabled` is true, NetGoat selects certificates by SNI in this order:
an exact streamed or local domain certificate, a one-label wildcard certificate,
an explicitly configured ACME certificate, then the static certificate pair.
Certificate reloads are atomic and retain the last valid certificate for a
domain if a replacement record is malformed.

Automatic ACME issuance is intentionally explicit. HTTP-01 requires port 80
to reach `ssl.acme.http_port`, and only names in `ssl.acme.domains` are allowed
to trigger issuance. The encrypted cache key is not stored in YAML.

```yaml
ssl:
  enabled: true
  port: ":443"
  acme:
    enabled: true
    accept_tos: true
    email: ops@example.test
    domains: [api.example.test]
    cache_dir: ./database/acme
    http_port: ":80"
```

Set `NETGOAT_ACME_CACHE_KEY` to a random base64-encoded 32-byte value before
starting the agent. The cache uses authenticated AES-256-GCM encryption and
owner-only files. Keep this key stable across restarts; losing it prevents the
agent from reading existing ACME account and certificate state. Use an ACME
staging directory URL before a production rollout to avoid CA rate limits.

HTTP-01 does not issue wildcard certificates. Supply a static or streamed
wildcard certificate when one is required.

## Dynamic JavaScript and TypeScript rules

Rules are ordered administrator-managed source units. They receive a frozen
request snapshot and may return `null` to continue, or exactly
`{ action: "allow" | "block", reason?: string }`. The first decision wins.
An `allow` decision allows the normal WAF and later proxy checks to continue;
it never bypasses them.

```yaml
dynamic_rules:
  enabled: true
  max_execution_milliseconds: 25
  rules:
    - name: block-export
      language: typescript
      source: |
        export function evaluate(request) {
          return request.path === "/export" && request.method !== "GET"
            ? { action: "block", reason: "exports are read-only" }
            : null;
        }
```

The engine has no filesystem, process, environment, network, HTTP, or Go
object bindings. It bounds source, compiled code, input, decision output, rule
count, and execution time. Invalid configuration, timeouts, request-body
overflow, and evaluation errors block the request and leave the previous
successfully compiled configuration active after a live update. See
[`internal/dynamicrules/README.md`](../internal/dynamicrules/README.md) for
the remaining in-process runtime boundaries.

## Cloudflare Access and reconciliation

Cloudflare Access is disabled by default. When `cloudflare.access.enabled` is
true, every request to the agent's main listener must carry a valid signed
Access assertion for the configured HTTPS issuer and one configured audience.
The JWKS cache is bounded and refreshes for normal key rotation; an invalid,
missing, or unavailable assertion receives a 403 and never reaches a handler.
Enable it only after the agent's own administrative endpoints are included in
the Access application.

Cloudflare DNS and tunnel changes are a separate startup-only reconciliation
plan. It is bounded to 32 operations, validates all identifiers and payloads
against a forced dry run before performing any real mutation, and defaults to
`dry_run: true`. Set `dry_run: false` only after reviewing the plan. The token
is read exclusively from `CLOUDFLARE_API_TOKEN`; do not put it in YAML.

```yaml
cloudflare:
  reconciliation:
    enabled: true
    dry_run: true
    account_id: "0123456789abcdef0123456789abcdef"
    dns_records:
      - zone_id: "abcdef0123456789abcdef0123456789"
        record:
          type: A
          name: app.example.test
          content: 203.0.113.10
    tunnels:
      - tunnel:
          name: netgoat
          config_src: cloudflare
```

Omit `record_id` or `tunnel_id` to create the corresponding resource; supply
one to update it. Deletion requires both `delete: true` and an explicit ID.
There is no background retry or autonomous discovery loop: a failed startup
plan is logged while the proxy keeps serving, and a new run requires an
intentional restart. Give the API token only the Cloudflare permissions needed
by the declared operations.

## Middleware SDK

See [`middleware-sdk.md`](middleware-sdk.md). The SDK is deliberately for
trusted, compiled-in extensions only. Do not use native Go plugins or treat it
as an isolation boundary for user-provided code.

## Production deploy

This is the terminate-TLS checklist for a host that will accept public
traffic. High-level reporting and default-hardening notes stay in
[SECURITY.md](../SECURITY.md). Version tags and the alpha→beta gate are in
[release.md](release.md).

Do this **before** adding routes. The shipped `config.yml` has
`routes: {}` on purpose: a fresh process returns `404` for every Host
instead of becoming an unauthenticated reverse proxy to loopback.

1. **TLS terminate.** Set `ssl.enabled: true` and a public `ssl.port`
   (typically `:443`). Supply a static `ssl.cert_file` / `ssl.key_file`
   pair, streamed certificates, or opt-in ACME (see [TLS and ACME](#tls-and-acme)).
   Fingerprint v1 and session-bound PoW run only when this process
   completes the TLS handshake. Cloudflare or another terminator in
   front means no `stack_class` and no PoW — same emit-nothing rule as
   the README.
2. **Listen addresses.** `listen` is the plaintext HTTP bind. Loopback
   (`127.0.0.1`, `::1`, `localhost`) is the only plaintext bind allowed
   without `allow_insecure_public_http: true`. An omitted `listen` is
   still `:8080` and is refused on a public address. Do not set the
   insecure flag on a production edge.
3. **Bootstrap auth before public routes.** Fresh databases have no
   default password. Export `NETGOAT_BOOTSTRAP_USERNAME` and
   `NETGOAT_BOOTSTRAP_PASSWORD` (at least 12 characters, unique, not a
   placeholder) and set `auth.enabled: true` on first start. Enable
   auth and TLS **before** adding a route that points at a loopback or
   private upstream. Metrics paths skip agent login (see below); scrape
   them from a network that is not the public listener.
4. **Empty routes, then add.** Leave `routes: {}` until the listener,
   TLS, and auth decisions are in place. Then add only the Hosts you
   intend to expose. A route to `127.0.0.1` publishes that process to
   every client that can reach this agent.
5. **`trusted_proxies`.** Forwarding headers are ignored unless you
   list the direct proxy hops that may set them. Keep the list narrow
   (the proxy's address or CIDR). Do not trust the public internet.
6. **Secrets stay out of git.** Set these in the process environment,
   not in committed YAML:

   | Secret | Role |
   | --- | --- |
   | `NETGOAT_CHALLENGE_SECRET` (preferred) or `DiamondKey` | HMAC key for PoW commitments. If both are unset the key is ephemeral and the verified set dies on restart. |
   | `NETGOAT_ACME_CACHE_KEY` | Encrypts ACME account and certificate cache. Keep it stable across restarts. |
   | `NETGOAT_BOOTSTRAP_PASSWORD` / `NETGOAT_BOOTSTRAP_USERNAME` | First local user only; unused once the user table is populated. |
   | `API_STREAM_KEY` | Overrides `api.key` for the control plane. |
   | `CLOUDFLARE_API_TOKEN` | Cloudflare reconciliation only; never a YAML field. |
   | TLS private keys, `.env`, SQLite files, recovery snapshots, telemetry ingest keys | Operator-held; listed in `.gitignore` for a reason. |

   `bot_auth.pinned_directories` must be `https` JWKS URLs you chose.
   An empty pin list is valid: unsigned clients take the PoW lane.

A missing `config.yml` is fatal. Copying the sample file onto a public
bind without TLS is also fatal unless you opted into insecure HTTP.

## Metrics and alerts

Metrics are off in the shipped config (`metrics.enabled: false`). When
enabled, the same listener serves JSON at `metrics.path` (default
`/__netgoat/metrics`) and Prometheus text at `<path>.prom` (default
`/__netgoat/metrics.prom`). Those paths are answered before route and
login handling. Do not expose them to the public internet; scrape from
loopback, a trusted proxy, or a locked-down path.

JSON fields that matter for a first scrape: `requests`, `responses`,
`blocked`, `proxy_errors`, `status_codes`, `block_reasons`,
`error_status_codes`, `recent_errors`, `uptime_seconds`. Prometheus
names are `netgoat_*_total` plus `netgoat_responses_by_status_total`
and `netgoat_blocks_by_reason_total`.

There is no separate PoW-issued counter. Challenge activity shows up as
`block_reasons` / `netgoat_blocks_by_reason_total` (`zero-trust`,
`waf:<rule>`, `rate-limit`, and the classifier names) and as 403s on
`/__netgoat/verify` failures. Watch process logs for
`Challenge verification failed` if you need the verify path itself.

Minimal alerts — use whatever already scrapes Prometheus or polls JSON.
This is not an observability product.

| Signal | What to watch | Why |
| --- | --- | --- |
| Process down | Scrape fails or `uptime_seconds` resets unexpectedly | The agent is the edge. |
| Elevated 5xx | `status_codes` / `netgoat_responses_by_status_total` for 500–599 | Origin or agent faults. |
| Proxy errors | `proxy_errors` / `netgoat_proxy_errors_total` rising | Upstream connect/reset/timeout. |
| Challenge / PoW | `zero-trust` block rate or verify-403 spike vs your baseline | Issue or fail storm; empty pin list means most bots should PoW, not skip. |
| WAF blocks | `waf:*` in `block_reasons` far above baseline | Rule hit or a bad deploy of expressions. |

Tune thresholds on a real host after you have a quiet baseline. Optional
`telemetry` is a separate opt-in ship of aggregates; leave it disabled
unless you operate that destination. The control-plane dashboard is not
embedded in this agent.
