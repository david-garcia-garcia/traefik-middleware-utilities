## Context

See proposal.md Why. Dest already table-drives `live_test.go` on env addrs and skips under `-short`. Unit tests pass with no LIVE env. Pester is a separate compose/Traefik job.

## Goals / Non-Goals

**Goals:**
- Split GitHub Actions so unit `test` has no service containers.
- One `e2e` job owns Redis 7 + Dragonfly and all LIVE env.
- Shared skip/fail helper: both unset or `-short` skip; exactly one addr fail; both set run every case on both engines.
- Expand live files to the engine-success paths in the specs.
- Write `knowledge/devdocs/std_go_test-suites.md` plus index row.

**Non-Goals:**
- Build tags.
- Renaming the Pester job.
- AUTH/SELECT live Redis (`--requirepass`).
- Extra engine images.
- Changing limiter or SimpleRedis runtime APIs.

## Decisions

1. **Keep dest skip knobs (`-short` + env), not `//go:build live`.** Specs already SHALL skip under `-short`. Alternative: build tags would force `-tags live` on every local e2e and break existing docs.

2. **Job id `e2e`, name `Go E2E`.** Pester stays `integration` / `Integration Tests`. Alternative: `go-integration` collides with the existing job name in GitHub’s check list.

3. **Per-package table + one fail-if-one-addr helper.** Copy the dest `backends` slice; after the loop, if not short and exactly one addr set, `t.Fatal`. Alternative: require both addrs always would make local `go test` without Docker fail.

4. **NOSCRIPT live via `SCRIPT FLUSH` then Eval.** Dest fake injects NOSCRIPT. Live engines need a flush. Alternative: a unique never-loaded digest — Eval already hashes the body; FLUSH is the documented Redis way.

5. **E2E `go test` timeout 5m.** Dest unit job is 2m. More live cases × two engines + Yaegi GOPATH copies. Alternative: keep 2m and discover CI timeouts later.

6. **Usage packet `std_go_test-suites.md` under existing `std`/`go`.** Alternative: `build` root would add an allowlist domain this run did not need.

## Risks / Trade-offs

- [SCRIPT FLUSH races sibling tests on the shared CI Redis] → unique script body; EVALSHA miss is retried by Eval.
- [Key collisions on shared Redis] → keys from `t.Name()` as dest live files already do.
- [5m still too short] → bump the e2e job timeout only; do not slow the unit job.
- [One-addr fail surprises local developers with a single LIVE var] → README and the catalog packet state both addrs or neither.

## Migration Plan

Land on `master` via PR 26. No runtime deploy. Rollback: revert the workflow job split; live files remain skip-gated.

## Open Questions

None. Suite split, skip/fail, coverage, Yaegi, and packet name are assumed on `devstate/explore.md`.
