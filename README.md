# traefik-middleware-utilities

Shared Go libraries for Traefik middlewares.

Traefik loads local and catalog plugins through [Yaegi](https://github.com/traefik/yaegi), an interpreter. This repo exists so the pieces those middlewares keep rewriting — Redis, a reclaim table, and a leaky bucket — live in one place and stay inside the subset of Go that Yaegi can run.

## Libraries

Introduced one at a time.

| Library | Status | Role |
| --- | --- | --- |
| Reclaim table | Current | In-process table that tracks entries and reclaims them when they expire or are released. |
| SimpleRedis | Current | Shared stdlib RESP client (GET/MGET/SET/DEL/INCR/EXPIRE/EVAL) middlewares import instead of inventing one. Apache-2.0 (copied from crowdsec-bouncer). |
| Leaky bucket | Planned | Rate-limit primitive used by middlewares that need a classic leaky-bucket clock. |

## Yaegi

This code is interpreted inside Traefik, not compiled into it. Treat that as a hard constraint, not a later port.

Rules of thumb:

- Prefer the Go standard library. Every import is also interpreted.
- No cgo, no `unsafe`, no assembly, no `//go:embed` unless Yaegi in the target Traefik version is known to support it.
- Avoid features the Traefik Yaegi build does not implement (some generics, some `reflect`, runtime-heavy packages).
- Keep APIs simple: concrete types, explicit methods, no clever init-time magic.
- Prove each package with tests that a normal `go test` run covers, then sanity-check the same sources under Yaegi before calling a library done.

If a change would be fine in compiled Go but fails under Yaegi, the Yaegi failure wins.

## Layout

```text
reclaim/      reclaim table
simpleredis/  stdlib RESP client (Apache-2.0)
e2e/          fake Traefik plugins + Pester harness (Yaegi)
bucket/       leaky bucket (later)
```

Module path: `github.com/david-garcia-garcia/traefik-middleware-utilities`.

## Tests

```text
go test ./reclaim/...
go test ./simpleredis/...
./Test-Integration.ps1
```

`Test-Integration.ps1` starts Traefik v3.7.11 with fake local plugins (`e2e/reclaimprobe`, `e2e/simpleredisprobe`) so reclaim and SimpleRedis run under Yaegi. Docker is required. The copied SimpleRedis client is Apache-2.0 (`simpleredis/LICENSE`).

CI (`.github/workflows/ci.yml`) runs golangci-lint, `go test -v ./...`, and that same Pester harness on every pull request and on pushes to `master`.

Tag a version (`v1.0.0`) to cut a GitHub release via GoReleaser (source archive + SBOM; no plugin binary).
