# Explore
IssueKey: 2026-09-11-reclaim-afterfunc

Verdict: in progress

## Concepts

`reclaim` binds one holder context per successful `Open` and today parks a waiter until that context is done:

```
  Open (put / bind / reclaim)
       │
       └─ go t.watch ── waitCtx ──► drop
              │
              ├─ Done() != nil  →  <-done          (parked goroutine for the hold)
              └─ Done() == nil  →  poll Err() 20ms (nilDoneCtx, Background)
```

Traefik `New` ctx is `WithCancel` (`knowledge/devdocs/std_go_reclaim.md`). That path is `Done() != nil`. The Yaegi-shaped holder (`nilDoneCtx` in `reclaim/table_test.go`) and `context.Background()` are `Done() == nil`; spec `std_go_reclaim_context-lease` keeps those live until `ctx.Err()` is set.

`context.AfterFunc` (Go 1.21, this module's language pin) registers `f` on a cancellable context without a parked waiter. When the context is done it still `go f()`. If `Done()` is nil it never runs `f` (`GOROOT/src/context/context.go` `propagateCancel`: `done == nil` returns; `afterFuncCtx.cancel` is the `go a.f()`).

Usage packet `knowledge/devdocs/std_go_reclaim.md` is enough to *call* `Open`. AfterFunc is an internal wait change; Language has no gap. Research: `knowledge/research/ext_traefik_plugins_yaegi-afterfunc/` (symbol map + interp call).

No identity reconstruction (client address, user, tenant, Host). Holders still pass Traefik `New` ctx.

Watch spawn sites (searched `reclaim/*.go` for `go t.watch` / `waitCtx`): `reclaim/table.go:187` bind, `:240` put, `:269` reclaim. `watch` is `:274`. `waitCtx` is `:114-123`. Package `Open` (`reclaim/default.go:27-28`) forwards to `Default().Open` (same three sites). No other product `watch` callers. `e2e/reclaimprobe/plugin.go:55` calls `reclaim.Open` only.

```
  Done() != nil (Traefik WithCancel / timeout)
       Open ── AfterFunc(ctx, drop)     // no parked waiter; f runs in a goroutine at cancel
  Done() == nil (nilDoneCtx, Background)
       Open ── go watch / waitCtx poll  // AfterFunc would never run f
```

## Reproduction

Claimed cost **reproduced**. Not a crash: current `Open` parks one goroutine per cancellable holder.

Throwaway only (`%TEMP%/reclaim-afterfunc-probe`, not in the product tree). Host `go1.25.6 windows/amd64`. Existing compiled suite `go test ./reclaim/ -count=1` — **pass** (1.066s), including `TestTable_HolderWithoutDoneChannelIsPolled` and `TestTable_GoroutinesReturnToBaseline`.

### 1. Compiled AfterFunc vs `watch`

50 `WithCancel` holders, hold without cancel, then cancel. `waitCtx` is the table's function copied into the probe.

| Path | goroutines during hold (delta vs baseline 1) | callbacks fired | after cancel |
|---|---|---|---|
| `go waitCtx` (today's `watch`) | **+50** (`watchHold=51`) | 50 | back to 1 |
| `context.AfterFunc` | **+0** (`afterHold=1`) | 50 | 1 |

AfterFunc ran `f` off the caller (`mainGID=1`, `lastAfterGID=117`). It avoids the parked waiter during the hold; it still starts a goroutine at cancel time.

### 2. Yaegi v0.16.1 interp call

GOPATH interp, `interp.New(Options{GoPath})`, `Use(stdlib.Symbols)` only (no unsafe; matches `docker-compose.yml` `useunsafe=false` and `reclaim/yaegi_test.go`). Interpreted package `afterprobe` called `context.AfterFunc` on `WithCancel`, canceled, observed the callback.

**Pass:** `interp AfterFunc call: ok` (no eval/import error).

### 3. Nil `Done()` must not use AfterFunc

Same compiled probe, `nilDoneCtx` matching `reclaim/table_test.go:294-317` (`Done()` nil; `cancel` sets `Err()`):

- `nilDoneAfterFuncFired=false` after `cancel` + 250ms
- `backgroundAfterFuncFired=false`
- already-canceled `WithCancel`: `alreadyCanceledAfterFuncFired=true` (stdlib: if already done, `f` runs immediately in its own goroutine)

`TestTable_HolderWithoutDoneChannelIsPolled` would hang or never dispose if the table used AfterFunc on that holder. Keep the poll fallback.

## Decisions

**Ship the `Done() != nil` branch; keep the nil-`Done` poll.** Yaegi can call AfterFunc. The compiled hold is the cost the ticket named. Out of scope forbids adopting AfterFunc without the spike, replacing the nil-Done poll, and upgrading Traefik/Yaegi. This is the gated prototype the requirement asked for, not a note/debt.

**Register `drop` via AfterFunc; do not store the stop func.** `drop` already returns when `state != slotAwake` (`reclaim/table.go:292-294`). `Reset` sets awake slots to `slotGone` (`:377`). `TestTable` stale-holder case (`table_test.go:1256-1260`) requires a late watcher not to sleep the next incarnation. Same as today's parked `watch` after Reset. Stop storage would be a new per-holder field nobody asked for.

**No spec wait-mechanism rewrite.** `std_go_reclaim_context-lease` names holder Done / `ctx.Err()` and that table goroutines exit (`spec.md:26-27`, `:172-188`). It does not name `watch` or `<-Done()`. AfterFunc still runs `f` in a goroutine at cancel; that goroutine must exit like `watch` (existing `TestTable_GoroutinesReturnToBaseline`). Propose may mention the branch in design; do not add an AfterFunc SHALL.

**Do not change `Open`, grace, hooks, or logging.** Internal wait only. Callers keep passing Traefik `New` ctx.

**Prove AfterFunc in `reclaim` tests, not a new Pester case.** Compiled: hold-time goroutine coverage for cancellable holders (today's baseline test is post-cancel). Interp: existing GOPATH harness (`reclaim/yaegi_test.go`) will load `table.go` once it names AfterFunc; keep `useunsafe` false. Pester already loads interpreted `reclaim` through Traefik; do not add an AfterFunc-specific e2e.

## Open questions

- Q: Can interpreted `reclaim` (Yaegi v0.16.1, `useunsafe` false) call `context.AfterFunc` end-to-end?
  Rank: additive asked — new wait call this change would add; Unknowns: "Whether interpreted reclaim can call context.AfterFunc end-to-end"; Desired: "Confirm Yaegi v0.16.1 can interpret a reclaim that calls context.AfterFunc before adopting"
  Decision: resolved — yes. Throwaway GOPATH interp (`%TEMP%/reclaim-afterfunc-probe/yaegi`), Yaegi v0.16.1, `Use(stdlib.Symbols)` only: `interp AfterFunc call: ok`.
  By: explore

- Q: Ship AfterFunc in product `reclaim`, or note it as debt without a hunk?
  Rank: bounded asked — existing wait unit `watch`/`waitCtx` in `reclaim/table.go`; spawn sites enumerated: `:187`, `:240`, `:269` (plus `watch` `:274`, `waitCtx` `:114-123`); all migrate here. Desired: "Prototype replacing the parked watch goroutine with context.AfterFunc when ctx.Done() != nil"; ticket: "worth prototyping, not worth doing blind"
  Decision: resolved — ship. `Done() != nil` → `context.AfterFunc(ctx, drop)`; `Done() == nil` → keep `go watch` / `waitCtx` poll. Not a note.
  By: explore

- Q: Is AfterFunc's stop func required on `Reset` / stale-incarnation drop, or is registering `drop` enough?
  Rank: additive incidental — optional per-holder stop storage this change could add; listed under Unknowns, not an In-scope or acceptance line; Out of scope does not name stop
  Decision: assumed — do not store stop. Register `drop`. `drop` no-ops when `incarnation.state != slotAwake`; Reset marks gone; stale-holder test already requires that late fire does not touch the next incarnation. Apply called AfterFunc and discarded the stop func.
  By: implement

- Q: Which Go build tag Traefik v3.7.11's Yaegi uses (`go1.21` vs `go1.22` extracts)?
  Rank: additive asked — Unknowns: "Which Go build tag Traefik v3.7.11's Yaegi uses"; Out of scope: "Upgrading Traefik or the Yaegi pin"
  Decision: resolved — both extracts map `AfterFunc` (`stdlib/go1_21_context.go` `go1.21 && !go1.22`, `stdlib/go1_22_context.go` `go1.22`; no `go1_23_context.go`). Host Go 1.25.6 selects the `go1.22` extract. Interp call passed on that extract. Pin stays v0.16.1.
  By: explore

- Q: Does `std_go_reclaim_context-lease` need a wait-mechanism delta (AfterFunc vs `<-Done` / poll)?
  Rank: additive incidental — spec already states holders go Done / `Err()` and goroutine exit, not `watch`; Desired: spec only "if the wait mechanism is specified beyond holders go Done"
  Decision: assumed — no mechanism SHALL. Keep nil-Done poll requirement and goroutine-exit. AfterFunc is how cancellable wait is implemented. Propose may note it in design.md.
  By: explore

- Q: Who already owns client address / user / tenant / Host for this table?
  Rank: additive asked — workflow identity question; this change does not emit those facts
  Decision: resolved — none. The table stores caller keys and `any`. It does not reconstruct request identity. Holders pass Traefik `New` ctx; that ctx is Traefik's.
  By: explore

- Q: Where is the product proof that interpreted `reclaim` loads AfterFunc (in-process Yaegi vs Pester)?
  Rank: additive asked — Affected: "Yaegi interp / Pester e2e if the spike says the interpreted package must load AfterFunc"
  Decision: resolved — `reclaim/yaegi_test.go` GOPATH + `stdlib.Symbols` (`useunsafe` false) loaded `table.go` after AfterFunc landed. `go test ./reclaim/...` passed. No new Pester AfterFunc case. Pin stays v0.16.1.
  By: implement
