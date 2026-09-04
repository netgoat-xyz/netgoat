# Release train

How this agent is versioned, tagged, and judged ready to leave alpha.
Feature behavior belongs in the [README](../README.md) and
[operations guide](operations.md). Vulnerability reporting stays in
[SECURITY.md](../SECURITY.md).

## Version and tags

- Product versions are [SemVer](https://semver.org/): `MAJOR.MINOR.PATCH`
  with an optional prerelease (`alpha.1`, `beta.1`).
- Git tags **must** start with `v`: `v0.1.0`, `v0.1.0-alpha.1`,
  `v0.1.0-beta.1`.
- [`.github/workflows/release-agent.yml`](../.github/workflows/release-agent.yml)
  triggers on `push` tags matching `v*`, or on `workflow_dispatch` with an
  explicit `v…` tag name. A tag without the prefix does not run that
  workflow.
- Use [Conventional Commits](https://www.conventionalcommits.org/) on
  the default branch (see [CONTRIBUTING.md](../CONTRIBUTING.md)). That
  is project convention, not a CI-enforced check. `fix:` and `feat:`
  feed patch and minor; a `BREAKING CHANGE` footer or `!` after the type
  feeds major. Alpha and beta cuts may still increment the prerelease
  identifier without a major bump.

### Historical tag `0.0.1-alpha.1`

The only existing git tag is `0.0.1-alpha.1`. The GitHub Release title
and README call the same cut `v0.1.0-alpha.1`. That tag does not match
`v*`, so today's release CI would not fire if it were pushed again.

Leave `0.0.1-alpha.1` where it is. Do not delete it, move it, or add a
second tag on the same commit from a docs PR. The next published cut
creates a **new** `v…` tag.

## Changelog

[CHANGELOG.md](../CHANGELOG.md) follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

- Land operator-visible work under `[Unreleased]`.
- When cutting a release, rename that section to the new version and
  date, and open a fresh `[Unreleased]`.
- The alpha section documents the `0.0.1-alpha.1` / `v0.1.0-alpha.1`
  mismatch. Do not rewrite it to pretend the tag was always `v…`.

`release-content.md` is the packaged archive blurb, not the changelog.
Keep version and secret names there aligned with this file when they
appear. Release notes rendered by the workflow prepend the tag and append
asset checksums.

## How to cut a release

Maintainers (Duckey / Senior SWE) cut tags. A documentation PR does not
push tags or GitHub Releases.

1. `main` is green: `go vet ./...` and `go test -race ./...` on the
   latest commit (PR CI in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)).
2. Move `[Unreleased]` in `CHANGELOG.md` to `## [vX.Y.Z] - YYYY-MM-DD`
   (or `vX.Y.Z-beta.N`). Leave a new empty `[Unreleased]`.
3. Align the README warning, `docs/release.md` "current label" if it
   changed, and any version mention in `release-content.md`.
4. Tag the intended commit: `git tag -a vX.Y.Z[-prerelease] -m "vX.Y.Z[-prerelease]"`
   and push that tag. The `v` prefix is what starts release-agent.
5. Confirm the workflow published archives, checksums, and a GitHub
   Release whose **tag** and **title** both use the same `v…` name.
   Prerelease tags (those containing `-`) stay marked prerelease and are
   not `latest`.

`workflow_dispatch` can publish an already-created `v…` tag. Prefer a
pushed tag so CI and the Release stay on the same ref.

## Current alpha label

Public docs call this **`v0.1.0-alpha.1`**. That is the product label.
The git object that marks the original cut is still `0.0.1-alpha.1`.

## Alpha → beta exit criteria

All of the following must be true on `main` before a `v…-beta.1` (or
later beta) tag. This is a written gate, not a vibe.

- [ ] Public HTTP refuse-without-TLS and missing-config-fatal remain
      green on `main` (`startup_config_test.go` / [#111](https://github.com/netgoat-xyz/netgoat/pull/111)).
- [ ] PR CI (`go vet` + `go test -race`) is green on `main`
      ([#110](https://github.com/netgoat-xyz/netgoat/pull/110),
      [#112](https://github.com/netgoat-xyz/netgoat/pull/112)).
- [ ] Challenge path is session-bound PoW plus pinned Web Bot Auth skip.
      No text, click, or slider puzzles
      ([#114](https://github.com/netgoat-xyz/netgoat/issues/114),
      [#116](https://github.com/netgoat-xyz/netgoat/pull/116)).
- [ ] Fingerprint emit-nothing rules are documented (README honesty +
      `internal/fingerprint`) and tested: no JA4 / `stack_class` for
      plaintext HTTP, TLS pass-through, or Cloudflare-in-front
      (`CF-Connecting-IP`). No forge of another terminator's ClientHello
      ([#113](https://github.com/netgoat-xyz/netgoat/issues/113),
      [#117](https://github.com/netgoat-xyz/netgoat/pull/117)).
- [ ] Production deploy guide in [operations.md](operations.md) has been
      followed on a real terminate-TLS host: empty `routes: {}` default,
      bootstrap auth before public routes, metrics scrape exercised.
- [ ] No open P0 security issues on this agent.
- [ ] Changelog updated and the beta cut uses a `v*` tag so release-agent
      runs. Tag name and GitHub Release title match.

### Not required for beta

These may stay planned, optional, or incomplete:

- Cloudflare feature parity with a full Cloudflare-in-front deploy
- VSA ([#98](https://github.com/netgoat-xyz/netgoat/issues/98))
- HTTP/3 / QUIC fingerprint (documented hole in v1)
- Fancy or default-on AI classifiers (GoatAI / Koda remain optional)
- Control-plane MVP (this repo is the agent; streamed snapshots stay
  optional)

Beta is still prerelease. It is not a promise of Cloudflare-scale
bot management or an identity product. `stack_class` is a TLS/HTTP stack
class: Chrome on a million laptops will collide. Do not persist it as a
user.
