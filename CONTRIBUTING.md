# Contributing to NetGoat

Thank you for helping improve NetGoat. This repository contains the Go reverse-proxy agent; companion services may use different toolchains and document their own commands.

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Report vulnerabilities privately according to [SECURITY.md](SECURITY.md), not in a public issue.

## Development setup

1. Install Go 1.24 or newer and a C toolchain for the SQLite driver.
2. Fork and clone the repository.
3. Create a focused branch such as `fix/cache-revalidation` or `feat/routing-policy`.
4. Keep secrets, generated databases, recovery snapshots, model files, and telemetry identifiers out of Git.

Run the agent with `go run .`. Optional AI workers require their own Python model dependencies, but disabled workers are not needed for the Go test suite.

## Changes and tests

- Keep a change focused and preserve backward compatibility unless the proposal explicitly calls for a breaking change.
- Add a regression test for bug fixes and tests for new behavior.
- Use comments where they explain security assumptions, concurrency, protocols, or non-obvious tradeoffs; avoid comments that merely repeat the code.
- Avoid new dependencies when the standard library or an existing dependency is sufficient.
- For hot-path changes, consider allocations, unbounded cardinality, lock contention, and attacker-controlled body sizes.

Before opening a pull request, run:

```sh
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

If you changed Python workers, also run:

```sh
python3 -m py_compile ai/*.py
```

## Commits and pull requests

Use [Conventional Commits](https://www.conventionalcommits.org/) with a useful scope, for example:

```text
fix(proxy): preserve upstream base paths
perf(waf): compile expressions during configuration reload
docs(readme): distinguish shipped features from roadmap
```

Explain why the change is needed, its security or performance impact, and how it was verified. Link the relevant issue or discussion when one exists. Maintainers may ask that a large feature be discussed before implementation so its configuration and compatibility surface can be agreed first.

By contributing, you agree that your contribution is licensed under this repository's [AGPL-3.0 license](LICENSE).
