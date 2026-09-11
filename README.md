# traefik-middleware-utilities

Shared Go libraries for Traefik middlewares.

Traefik loads local and catalog plugins through [Yaegi](https://github.com/traefik/yaegi), an interpreter. This repo exists so the pieces those middlewares keep rewriting — Redis, a reclaim table, and a leaky bucket — live in one place and stay inside the subset of Go that Yaegi can run.

## Libraries

Introduced one at a time. Only the reclaim table is in scope right now.

| Library | Status | Role |
| --- | --- | --- |
| Reclaim table | Current | In-process table that tracks entries and reclaims them when they expire or are released. |
| Redis connection | Planned | Shared Redis client setup that middlewares can reuse without each plugin inventing its own. |
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
reclaim/    reclaim table
e2e/        fake Traefik plugin + Pester harness (Yaegi)
redis/      Redis connection (later)
bucket/     leaky bucket (later)
```

Module path: `github.com/david-garcia-garcia/traefik-middleware-utilities`.

## Tests

```text
go test ./reclaim/...
./Test-Integration.ps1
```

`Test-Integration.ps1` starts Traefik v3.7.11 with a fake local plugin (`e2e/reclaimprobe`) so reclaim runs under Yaegi. Docker is required.

CI (`.github/workflows/ci.yml`) runs golangci-lint, `go test -v ./...`, and that same Pester harness on every pull request and on pushes to `initial`.

Tag a version (`v1.0.0`) to cut a GitHub release via GoReleaser (source archive + SBOM; no plugin binary).
