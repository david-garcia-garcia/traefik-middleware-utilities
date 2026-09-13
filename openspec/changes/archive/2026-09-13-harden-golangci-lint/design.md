## Context

See proposal.md Why. Dest `.golangci.yml` enables ten linters. CI `lint` uses `version: latest`. Explore re-measured dest `2847a81` with golangci-lint v1.63.4, issues uncapped. `gocritic.enabled-checks` adds to defaults. `unnamedResult.checkExported: true` is weaker (exported-only).

## Goals / Non-Goals

**Goals:**
- `golangci-lint run` clean on the new config.
- Compile-time names and comments only; Yaegi tests still pass.

**Non-Goals:**
- Runtime behavior change. Reordering `borrow`/`dial` to put `error` last. Enabling `testpackage`. `errors.Is` on the ten identity sites. `goimports`/`gofumpt`. golangci-lint v2.

## Decisions

1. **Pin `v1.63.4`.** Why: this host and every dest count. Alternative: `latest` — rejected (v2 schema). Alternative: another v1.x — not validated here.

2. **`gocritic.enabled-checks` adds `unnamedResult`; `checkExported: false`.** Why: dest measured 17 vs 3. Comment in `.golangci.yml` that `true` is exported-only. Alternative: replace the default set — would drop `assignOp`. Rejected.

3. **Name `borrow`/`dial` by hand; `nolint:revive` for `error-return`.** Why: unnamedResult does not flag `(*pooledConn, error, bool)` (only one ambiguous primitive). Reorder would collide with in-flight PRs. Alternative: put `error` last now — rejected (Desired / Out of scope).

4. **`//nolint:errorlint` on all ten dest `==` sites. Convert none.** Why: `err == errUnreachable` is the tcp-session identity requirement; the other nine are identity classifiers on sentinels this package or `ReadSlice` returns exactly. Reason 1 (Yaegi `errors.As` panic) does not apply on dest. Alternative: `errors.Is` on timeout/miss/issue — would change wrapping behavior; Desired forbids runtime change.

5. **Exclude `_test.go` from `forcetypeassert`.** Why: 14 dest hits are all tests (Yaegi `Eval` plus `value.(*box)`). Production stays a zero-hit ratchet.

6. **Omit `goimports`/`gofumpt`.** Why: `diff` missing on Windows; `gofmt` already on. Alternative: CI-only second config — rejected (one file).

7. **Leave `testpackage` off.** Record in `std_go_test-suites.md`. Tests reach unexported pool state.

8. **Uncap issues in config.** Dest default `max-same-issues: 3` hid counts. Set `max-issues-per-linter: 0` and `max-same-issues: 0` so the ratchet is real.

## Risks / Trade-offs

- [Risk] In-flight SimpleRedis PRs rewrite `commands_exec.go`, `pool.go`, `resp.go` → Mitigation: merge this after those branches; document in the PR body. Do not narrow scope.
- [Risk] Named results plus explicit `return` invite naked returns → Mitigation: `nakedret` enabled; keep explicit returns.
- [Trade-off] `revive` `error-return` stays suppressed on `borrow`/`dial` until a later reorder PR.
- [Trade-off] Windows contributors cannot enable goimports locally; format stays `gofmt`.

## Migration Plan

- Land config, mechanical fixes, catalog, pin in one PR.
- Rollback: revert `.golangci.yml` and `ci.yml` `version:` plus the mechanical Go edits (compile-compatible names).

## Open Questions

None. Explore Qs are resolved or assumed on `devstate/explore.md`.
