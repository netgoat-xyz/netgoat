<img width="5658" height="1600" alt="NetGoat" src="https://github.com/user-attachments/assets/d30fb971-4b39-490c-ac08-0d688e8f9ada" />

# NetGoat agent

NetGoat is a self-hosted reverse proxy and traffic-policy agent written in Go. It can run from a local YAML configuration, consume snapshots from the companion control plane, and continue serving the last known-good configuration during an outage.

> [!WARNING]
> NetGoat is active alpha software. Review the sample configuration, use strong bootstrap credentials, and place administrative services behind TLS before exposing a deployment to the internet.

## Feature status

| Capability | Status | Notes |
| --- | --- | --- |
| Domain and path routing | Available | Exact, wildcard, regex, and longest-prefix path routes; local routes can be overridden by streamed routes. |
| Load balancing and failover | Available | Round-robin pools, bounded concurrent health checks, and safe-method retry/failover. |
| WAF rules | Available | Precompiled expression rules with priorities, `BLOCK`/`ALLOW` actions, and request host/method/path/query/header context. |
| Traffic controls | Available | Global rate limiting, request queueing, bandwidth throttling, honeypot handling, and dynamic challenges. |
| Shared response cache | Available | Bounded LRU/TTL cache for explicitly public responses, with HTTP freshness and revalidation safeguards. |
| Local authentication | Available | Cookie or Basic authentication, per-user zero-trust challenge flags, and explicit secure bootstrap users. |
| TLS termination | Available | Static fallback, streamed per-domain/wildcard certificates, and atomic SNI selection. |
| WebSocket proxying | Available | Upgrade connections are preserved by Go's reverse proxy. |
| Metrics | Available | JSON and Prometheus endpoints for traffic, cache, block, latency, and proxy-error counters. |
| AI request classifiers | Optional | Local GoatAI, Koda-WAF, and Koda-2 workers; model files and Python dependencies are required only when enabled. |
| Control-plane recovery | Available | Polling with timeouts/backoff, atomic snapshot reconciliation, deduplication, and private on-disk recovery snapshots. |
| Operational telemetry | Optional | Explicitly opt-in delivery to the companion telemetry server, with endpoint and ingestion-key configuration. |
| Automatic certificate issuance/renewal | Available (opt-in) | Explicit ACME allow-list, HTTP-01 handler, encrypted persistent cache, and last-known-good certificate retention. |
| JavaScript/TypeScript dynamic rules | Available (opt-in) | Isolated, bounded JS/TS request decisions with atomic last-known-good reload and fail-closed evaluation. |
| Developer plugin catalog and middleware SDK | Available | Restart-only selections for exact compiled descriptors; v1 capability grants, lifecycle isolation, and no remote code/artifact loading. |
| Cloudflare Access, DNS, and tunnel management | Available (opt-in) | Fail-closed Access JWT/JWKS verification plus bounded, dry-run-by-default startup reconciliation using an environment-only token. |
| Per-route cache/bandwidth policies | Available | Route policies inherit global defaults, isolate cache/bandwidth state, and support explicit per-route overrides. |

The dashboard shown by the wider NetGoat project belongs to the control plane. This agent exposes metrics APIs but does not embed that dashboard.

## Quick start

Requirements:

- Go 1.25 or newer
- one or more reachable HTTP upstreams

Clone the repository, edit the sample `routes` in `config.yml`, then run:

```sh
go test ./...
go run .
```

The default listener is `:8080`. A minimal local route looks like this:

```yaml
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
