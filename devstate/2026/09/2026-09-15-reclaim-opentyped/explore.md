# Explore
IssueKey: 2026-09-15-reclaim-opentyped

## Concepts

```
Open(create, hooks)          ← compatibility wrapper; hooks evaluated before the call
        │
        ▼
OpenWithHooks(create→hooks)  ← owns the lookup loop; put stores hooks create returns
        │
        ▼
OpenTyped[T]                 ← package func; same create; returns T or mismatch error
```

- **Open**: create-once for a key. Today the loop lives here (`reclaim/table.go:369`). Hooks sit beside create, so they cannot close over the value unless the caller declares a variable first.
- **put**: unexported. Today `create() (any, error)` then `publishPut(..., hooks)` with the caller-supplied hooks (`reclaim/table.go:407-418`). `publishPut` already takes hooks as a plain value; the state machine does not change.
- **OpenWithHooks**: create returns `(any, Hooks, error)`. The returned hooks are what `EnforceCloseBeforeOpen` reads later — a bool, not a func, so an external helper cannot fake it.
- **OpenTyped[T any]**: function, not a method. `go 1.21` supports it (`go.mod`). Table stays non-generic (`knowledge/devdocs/std_go_reclaim.md` Language **Table**; `knowledge/research/ext_traefik_plugins_yaegi-generics/notes.md`).
- **Yaegi call expression**: instantiation must stay a call in a package that can name T. Package-level var / type alias / struct field of a foreign generic instantiation fails under Yaegi v0.16.1.
- **Error precedence of Open**: nil table (`reclaim: open %q: nil table`), then nil logger (`reclaim: open %q: nil logger`), then nil create (`reclaim: create %q: nil create`). Tests: `TestTable_NilTableOpenErrors`, `TestTable_NilOpenLoggerRejected`, `TestTable_NilCreateUnsticksKey`. No combined test today that a nil table plus nil create still reports nil table first.

## Decisions

- Keep `Open` signature, semantics, and error strings. It becomes a wrapper: if `create == nil`, report table/logger/create in that order; else wrap `create` + fixed `hooks` and call `OpenWithHooks`.
- Move the lookup loop into `OpenWithHooks`. Change `put` to `create func() (any, Hooks, error)` and drop its `hooks` parameter. Capture returned hooks; pass them to `publishPut`.
- Add `OpenTyped` in new `reclaim/opentyped.go`. On mismatch: zero T and `reclaim: open %q: want %T, got %T`. Keep the Yaegi call-expression constraint in the doc comment.
- Treat `D:\tmp\reclaim-upstream-patch\` as a starting point (`git apply -p11` or hand hunks). Match package doc-comment voice.
- Tests (compiled, in-package): OpenWithHooks honours `EnforceCloseBeforeOpen` from create and a later Open binds without re-running create; OpenTyped typed return, singleton identity, mismatch error; Open nil-argument precedence regression including the combined nil-table+nil-create case the wrapper must preserve.
- Yaegi: extend `reclaim/yaegi_test.go` with an `OpenTyped` **call expression**. Skip under `-race` like the other interp tests.
- Spec/usage: delta on existing `openspec/specs/std_go_reclaim_value-lifecycle` (hooks-at-put, later Open ignores its hooks argument). Update `knowledge/devdocs/std_go_reclaim.md` Language **Open** / **Hooks** and the pattern snippet so callers are not taught the closed-over variable as the only path.
- Do not migrate `e2e/reclaimprobe` or existing `Open` tests onto OpenTyped. Do not tag or bump a version. Consumer `traefik-geoblock` re-vendors after merge (out of this repo).
- Research packet `ext_traefik_plugins_yaegi-generics` already owns the Table-non-generic fact. Do not rewrite it this run. A stale sentence there still describes type-switch discovery; current `master` already uses `Hooks`. Not this ticket.

## Open questions

- Q: What OpenSpec change folder name, and which live spec leaf takes the delta?
  Rank: additive asked — requirement Desired names both functions; the hooks contract already lives on `std_go_reclaim_value-lifecycle`
  Decision: resolved — change kebab `reclaim-opentyped`; FindSpecHost fold into `std_go_reclaim_value-lifecycle` (high; candidates also `std_go_reclaim_context-lease`). Usage update on `std_go_reclaim.md`.
  By: propose

- Q: Does this run add a Yaegi harness case for OpenTyped?
  Rank: additive asked — requirement Unknowns and the ticket obligation name the harness if it exists
  Decision: resolved — yes. `reclaim/yaegi_test.go` is that harness. Add a call-expression `OpenTyped` case. Do not declare a package-level generic instantiation.
  By: explore

- Q: Do in-repo `Open` callers (`e2e/reclaimprobe`, existing table tests) migrate to OpenTyped?
  Rank: additive incidental — Desired is add functions; Open remains the compatibility surface
  Decision: resolved — no. New tests cover the new functions. Existing Open tests stay on Open.
  By: explore

- Q: What error strings does OpenWithHooks use for nil table, logger, and create?
  Rank: additive asked — requirement names Open error precedence
  Decision: resolved — the same three strings Open uses today, same order. Open's nil-create branch still reports table and logger first because the wrapping closure is never nil.
  By: implement
