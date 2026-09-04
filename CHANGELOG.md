# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Git tags going forward are `vX.Y.Z` or `vX.Y.Z-<prerelease>` (for example
`v0.1.0-beta.1`). That leading `v` is required so
[`.github/workflows/release-agent.yml`](.github/workflows/release-agent.yml)
runs. How to cut a release, and the alpha→beta exit criteria, live in
[docs/release.md](docs/release.md).

## [Unreleased]

Operator-facing work on `main` since the public alpha cut. This is not a
released version. The next cut must add a dated section here and a new
`v…` tag. Do not reuse or move the legacy tag `0.0.1-alpha.1`.

### Added

- Stream `/domains` records accept JSON `route_policy` as an alias of
  `policy`, so older and newer stream-server payloads both apply.
- Session-bound proof-of-work and pinned Web Bot Auth skip
  ([#114](https://github.com/netgoat-xyz/netgoat/issues/114),
  [#116](https://github.com/netgoat-xyz/netgoat/pull/116)). Challenge is
  JSON. Text, click, and slider puzzles are gone. Verify is bound to the
  terminated TLS session, not an IP. An empty `bot_auth.pinned_directories`
  list fails open to the PoW lane.
- Terminate-only stack fingerprint v1
  ([#113](https://github.com/netgoat-xyz/netgoat/issues/113),
  [#117](https://github.com/netgoat-xyz/netgoat/pull/117)). Opaque
  `stack_class` (`{JA4}|{akamai_h2}|{alpn_order}`) is emitted only when
  this agent terminates TLS. Plaintext HTTP, TLS pass-through, and
  Cloudflare-in-front (`CF-Connecting-IP`) emit nothing. WAF context and
  challenge difficulty can use `stack_class` mismatch; User-Agent alone
  does not raise difficulty. HTTP/3 / QUIC remains a documented hole.
- Release train and production ops floor: this changelog,
  [docs/release.md](docs/release.md), and a production deploy plus
  metrics/alert section in [docs/operations.md](docs/operations.md).

### Changed

- Operations guide documents connecting this agent to stream-server
  (`api.url` / `API_STREAM_KEY` → `/domains`) and last-known-good
  recovery when the companion control plane is down. Beta non-goals
  name an embedded dashboard, not a held companion CP.
- README feature status and honesty notes now distinguish shipped
  behavior from roadmap
  ([#115](https://github.com/netgoat-xyz/netgoat/pull/115), updated again
  by [#117](https://github.com/netgoat-xyz/netgoat/pull/117)). JA4 is live
  only on the terminate path. Session-bound PoW and pinned Web Bot Auth
  skip are live on that same path. VSA, H3/QUIC fingerprint, and
  Turnstile/reCAPTCHA are not shipped.

### Fixed

- Missing `config.yml` is fatal. Plaintext HTTP on a public address is
  refused unless `allow_insecure_public_http: true` is set
  ([#111](https://github.com/netgoat-xyz/netgoat/pull/111)).
- CI runs `go vet` and `go test -race` on pull requests and `main`, and
  the release test job matches that baseline
  ([#110](https://github.com/netgoat-xyz/netgoat/pull/110),
  [#112](https://github.com/netgoat-xyz/netgoat/pull/112)).

## [0.0.1-alpha.1] / product label `v0.1.0-alpha.1` - 2026-03-16

Current public alpha. Treat the product version as **`v0.1.0-alpha.1`**.

### Tag naming

| Surface | Value |
| --- | --- |
| Git tag (only tag in this repo) | `0.0.1-alpha.1` |
| GitHub Release title | `🚀 Release v0.1.0-alpha.1` |
| README / docs alpha label | `v0.1.0-alpha.1` |
| `release-agent.yml` tag filter | `v*` — this historical tag does **not** match |

The tag, the Release title, and the SemVer version (`0.0.1` vs `0.1.0`)
disagree. That is a legacy mismatch. **Do not delete or retag
`0.0.1-alpha.1`.** The next release is a new `v…` tag on the intended
commit, with a changelog section and the release-agent workflow.

The GitHub Release body for this cut is not a durable change list.
Operator-facing behavior since this cut is tracked under `[Unreleased]`.
