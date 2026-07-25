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

## Middleware SDK

See [`middleware-sdk.md`](middleware-sdk.md). The SDK is deliberately for
trusted, compiled-in extensions only. Do not use native Go plugins or treat it
as an isolation boundary for user-provided code.
