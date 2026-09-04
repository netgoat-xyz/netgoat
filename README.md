<img width="5658" height="1600" alt="NetGoat" src="https://github.com/user-attachments/assets/d30fb971-4b39-490c-ac08-0d688e8f9ada" />

# NetGoat agent

NetGoat is a self-hosted reverse proxy and traffic-policy agent written in Go. It can run from a local YAML configuration, consume snapshots from the companion control plane, and continue serving the last known-good configuration during an outage.

> [!WARNING]
> NetGoat is active alpha software (`v0.1.0-alpha.1`). The shipped
> configuration contains no routes. Review every upstream, use strong
> bootstrap credentials, and place administrative services behind
> authenticated TLS before exposing a deployment to the internet.

## Feature status

This table is the public roadmap surface. **Available** is what this agent
ships today. **Planned** is designed work with open specs — not live
behavior, not a package, and not an identity product.

The shipped `config.yml` has `routes: {}`. A fresh default deployment
returns `404` for every Host instead of proxying to a local service.

### Available

| Capability | Status | Notes |
| --- | --- | --- |
| Domain and path routing | Available | Exact, wildcard, regex, and longest-prefix path routes; local routes can be overridden by streamed routes. |
| Load balancing and failover | Available | Round-robin pools, bounded concurrent health checks, and safe-method retry/failover. |
| WAF rules | Available | Precompiled expression rules with priorities, `BLOCK`/`ALLOW` actions, and request host/method/path/query/header context. |
| Traffic controls | Available | Global rate limiting, request queueing, bandwidth throttling, honeypot handling, and dynamic challenges. Today's challenges are **text / click / slider** HTML puzzles in `internal/challenge`, scored in part from User-Agent and bound to IP — not PoW, not Web Bot Auth. |
| Shared response cache | Available | Bounded LRU/TTL cache for explicitly public responses, with HTTP freshness and revalidation safeguards. |
| Local authentication | Available | Cookie or Basic authentication, per-user zero-trust challenge flags, and explicit secure bootstrap users. |
| TLS termination | Available | Static fallback, streamed per-domain/wildcard certificates, and atomic SNI selection. Termination is required for the planned fingerprint and session-bound PoW; it does not emit JA4 today. |
| WebSocket proxying | Available | Upgrade connections are preserved by Go's reverse proxy. |
| Metrics | Available | JSON and Prometheus endpoints for traffic, cache, block, latency, and proxy-error counters. |
| AI request classifiers | Optional | Local GoatAI, Koda-WAF, and Koda-2 workers; model files and Python dependencies are required only when enabled. |
| Control-plane recovery | Available | Polling with timeouts/backoff, atomic snapshot reconciliation, deduplication, and private on-disk recovery snapshots. |
| Operational telemetry | Optional | Explicitly opt-in delivery to the companion telemetry server, with endpoint and ingestion-key configuration. |
| Automatic certificate issuance/renewal | Available (opt-in) | Explicit ACME allow-list, HTTP-01 handler, encrypted persistent cache, and last-known-good certificate retention. |
| JavaScript/TypeScript dynamic rules | Available (opt-in) | Isolated, bounded JS/TS request decisions with atomic last-known-good reload and fail-closed evaluation. |
| Developer plugin catalog and middleware SDK | Available | Restart-only selections for exact compiled descriptors; v1 capability grants, lifecycle isolation, and no remote code/artifact loading. |
| Cloudflare Access, DNS, and tunnel management | Available (opt-in) | Fail-closed Access JWT/JWKS verification plus bounded, dry-run-by-default startup reconciliation using an environment-only token. Cloudflare in front of this agent is **not** a JA4 source. |
| Per-route cache/bandwidth policies | Available | Route policies inherit global defaults, isolate cache/bandwidth state, and support explicit per-route overrides. |

Recent hardening (already merged): public plaintext HTTP is refused at
startup unless `allow_insecure_public_http: true` is set
([#111](https://github.com/netgoat-xyz/netgoat/pull/111)). CI runs `go vet`
and `go test -race` on pull requests and `main`
([#110](https://github.com/netgoat-xyz/netgoat/pull/110),
[#112](https://github.com/netgoat-xyz/netgoat/pull/112)).

The dashboard shown by the wider NetGoat project belongs to the control
plane. This agent exposes metrics APIs but does not embed that dashboard.

### Next / roadmap

Designed, not shipped. Specs live in the linked issues. Do not treat these
rows as implementation claims.

| Capability | Status | Notes |
| --- | --- | --- |
| Stack fingerprint v1 | Planned | JA4 + Akamai-style H2 `SETTINGS` / `WINDOW_UPDATE` / `PRIORITY` / pseudo-header order + ALPN order, **only when this agent terminates TLS**. Opaque `stack_class` for bot clustering — not a user, hardware ID, or canvas hash. No JA4H. HTTP/3 / QUIC is a documented hole. Spec: [#113](https://github.com/netgoat-xyz/netgoat/issues/113). |
| Challenge replace | Planned | Delete text / click / slider. Skip = pinned `Signature-Agent` JWKS + RFC 9421 web-bot-auth only. Cost = tiny session-bound PoW HMAC'd to the terminated TLS session with expiry in the MAC. Difficulty from load + `stack_class` mismatch, not User-Agent. JSON challenge, not DOM puzzles. Spec: [#114](https://github.com/netgoat-xyz/netgoat/issues/114). |
| VSA (Virtual System Administrator) | Planned | Out-of-band autonomous defense operator (not a hot-path classifier, not Kaseya VSA). Open: [#98](https://github.com/netgoat-xyz/netgoat/issues/98). |

**Honesty**

- Fingerprint is a client TLS/HTTP **stack class**. Chrome on a million
  laptops will collide. Do not persist it as identity.
- JA4 is **not live**. If Cloudflare or any other terminator sits in
  front, this agent must not fingerprint that ClientHello and call it the
  browser.
- Turnstile / reCAPTCHA are **not** NetGoat features.
- Session-bound PoW and pinned Web Bot Auth skip are **not live**.

**Gaps**

- No terminate (plaintext, pass-through, or another terminator in front)
  → no fingerprint and no PoW. Same rule as [#113](https://github.com/netgoat-xyz/netgoat/issues/113)
  and [#114](https://github.com/netgoat-xyz/netgoat/issues/114).
- Web Bot Auth registry is empty until operators seed a pinned allowlist.
- HTTP/3 / QUIC clients will not appear in fingerprint v1.
- Unsigned LLM browsers / `python` / `go-http-client` / curl take the
  **PoW** lane once [#114](https://github.com/netgoat-xyz/netgoat/issues/114)
  ships — not a magic skip.

**Hardening next** (docs only; not this change)

- Bind challenges to session, not IP (called out in
  [#114](https://github.com/netgoat-xyz/netgoat/issues/114); today's store
  keys on IP).
- Stop User-Agent bot scoring that flags the agents we want through
  (`bot` / `python` / `go-http-client` in `CalculateSuspicion`).
- Keep CI race + vet on PRs and `main` as the verification baseline.

## Quick start

Requirements:

- Go 1.25 or newer
- one or more reachable HTTP upstreams

Clone the repository, add the routes you intend to expose to `config.yml`,
then run:

```sh
go test ./...
go run .
```

The sample listener is `127.0.0.1:8080` (plaintext HTTP on loopback). A
minimal local route looks like this:

```yaml
listen: "127.0.0.1:8080"
auth:
  enabled: false

routes:
  app.localhost:
    type: domain
    targets:
      - url: http://127.0.0.1:3000
        health_check: http
```

Then send a request with the configured host:

```sh
curl -H 'Host: app.localhost' http://127.0.0.1:8080/
```

A missing `config.yml` is fatal. Plaintext HTTP on a public address
(`:8080`, `0.0.0.0`, `::`, or any non-loopback bind) is refused at startup
unless `allow_insecure_public_http: true` is set. Enable TLS for public
traffic. A route to a loopback or private-network target makes that target
reachable through NetGoat, so keep a public listener behind TLS or enable
authentication before adding such a route. The shipped `routes: {}` is
intentionally empty: a fresh default deployment returns `404` for every Host
instead of proxying to a local service.

If the control plane is unavailable, NetGoat uses local routes and then the last valid recovery snapshot. Configure `api.url` as an empty string for a fully offline deployment.

## Authentication bootstrap

Fresh databases do not contain a default password. To enable local authentication, set both bootstrap variables before the first start:

```sh
export NETGOAT_BOOTSTRAP_USERNAME=admin
export NETGOAT_BOOTSTRAP_PASSWORD='replace-with-at-least-12-characters'
```

Then set `auth.enabled: true`. Bootstrap credentials are used only when the user table is empty; existing users are not overwritten. Basic authentication does not create persistent cookie sessions.

## Configuration highlights

- `routes`: local fallback routes keyed by domain, wildcard/regex pattern, or path prefix; each route can override `policy.cache` and `policy.bandwidth`.
- `api`: control-plane URL, key, poll interval, timeout, and maximum retry interval.
- `health`: probe enablement, interval, timeout, and default path.
- `cache`, `rate_limit`, `request_queue`, `bandwidth`: bounded global traffic defaults; cache and bandwidth can be overridden per route.
- `metrics`: enables JSON at the configured path and Prometheus at `<path>.prom`.
- `listen`: plaintext HTTP bind address when TLS is off; the sample uses loopback.
- `allow_insecure_public_http`: explicit opt-in for plaintext HTTP on a public address.
- `ssl`: static fallback TLS, per-domain certificate selection, and optional ACME issuance/renewal.
- `dynamic_rules`: bounded administrator-managed TypeScript/JavaScript request decisions.
- `plugins`: restart-only catalog selections for middleware compiled into this exact agent build; `sha256` is the release descriptor fingerprint, not a downloaded artifact hash.
- `cloudflare`: optional Access JWT enforcement and explicit DNS/tunnel startup reconciliation. Reconciliation defaults to dry-run.
- `telemetry`: disabled by default; endpoint, shared ingestion key, and heartbeat interval.
- `anomaly`, `koda_waf`, `koda_2`: optional local inference workers.

Secrets may also be supplied through the environment. `API_STREAM_KEY` overrides the YAML control-plane key, `NETGOAT_ACME_CACHE_KEY` encrypts ACME state, and `CLOUDFLARE_API_TOKEN` is required for Cloudflare reconciliation. Do not commit `.env`, private keys, model files, databases, recovery snapshots, or telemetry identifiers.

See the [operations guide](docs/operations.md) for policy precedence, ACME setup, and dynamic-rule safety boundaries; the [middleware SDK guide](docs/middleware-sdk.md) for trusted compiled-in extensions; and the [developer plugin catalog guide](docs/developer-plugins.md) for the restart-time selection and publisher trust boundary.

## Architecture

```text
client -> NetGoat agent -> healthy upstream pool
              |    |
              |    +-> SQLite state + recovery snapshot
              +------> stream-server (optional control plane)
              +------> telemetry-server (optional, opt-in)
```

The agent's hot request path optionally verifies Cloudflare Access, applies authentication and traffic controls, resolves a route, evaluates dynamic rules, precompiled WAF rules, and any selected compiled middleware, optionally runs enabled local classifiers, and proxies the request. Health checks and control-plane polling run in bounded background workers.

The optional `docker-compose.yml` starts only a loopback-bound development MongoDB for `stream-server`; the Go agent itself does not require it. Export `MONGO_INITDB_ROOT_USERNAME` and `MONGO_INITDB_ROOT_PASSWORD` before running Compose so development credentials stay outside the repository.

## Development

Run the full Go verification suite before submitting changes:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Python worker syntax can be checked without installing their model dependencies:

```sh
python3 -m py_compile ai/*.py
```

See `CONTRIBUTING.md` for contribution conventions and `SECURITY.md` for private vulnerability reporting.

## Project links

- Community: [Discord](https://discord.com/invite/3aJ7MdJsZV)
- First donor: [Cozy Critters Society](https://opencollective.com/cozy-critters-society)
- License: [AGPL-3.0](LICENSE)

Special thanks to **Cozy Critters Society** and **Snow** for being NetGoat's first donors.
