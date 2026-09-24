# Explore
IssueKey: 2026-09-24-reclaim-alias

Verdict: in progress

## Concepts

Measured delta (baseline `origin/master` at `ba52347e91471b136d85e8dae247dee929992e65`, compared to crowdsec bouncer vendor `reclaim/`):

| Path | Change |
|------|--------|
| `reclaim/alias.go` | **add** (~245 lines): `SetAlias`, `Watch`, `ClearPublisher`, `Watcher` / `Box` / `Published`, alias teardown helpers |
| `reclaim/table.go` | **modify** (+58 / −5 vs master): `Table.aliases`, `slot.aliases`, `Peek` + `State`, `unbindIncarnationLocked` on incarnation teardown paths, `takeAll` clears aliases |
| `reclaim/opentyped.go` | **unchanged** (content identical after LF normalization) |

No other files under vendor `reclaim/` exist. Utilities `origin/master` already ships ten reclaim test files; none reference alias or `Peek` yet.

```
ownership key (Open holder)          public alias (weak Watch)
        │                                    │
        ▼                                    ▼
   slot.value ──SetAlias──► aliasEntry ◄── Watch(ctx, …)
        │                         │
        └─ slot.aliases[] ────────┘
```

- **Table** (`reclaim/table.go`): keyed incarnation store; this change adds forward map `aliases` and reverse links on each slot.
- **alias.go**: weak subscribers on a public name; strong refs stay on the ownership key from `Open`.
- **Peek**: read slot value and `Awake` / `Asleep` without binding or stopping grace; `ok=false` for missing, gone, or busy slots.
- **unbindIncarnationLocked**: when an incarnation ends (unmap, failed create, enforce close, grace dispose, `takeAll` / `Reset`), aliases pointing at it revert watchers to `empty` and clear publisher metadata — the measured “bug fix” is this wiring, not a separate one-line patch.

**In-repo call sites (utilities, `origin/master`):** searched `github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim` under the worktree — **2** import roots: `reclaim/*_test.go` (in-package tests) and `e2e/reclaimprobe/plugin.go` (Open-only probe). Neither uses alias or `Peek` today; adding APIs is backward compatible for them.

**Downstream consumer (out of this PR’s code scope):** crowdsec bouncer `pkg/reclaim` shim re-exports alias / `Peek` and carries compiled tests `pkg/reclaim/zzz_alias_test.go` and `pkg/reclaim/zzz_peek_test.go` against the vendored tree — use as the behavioral checklist for utilities tests, not as files to copy verbatim (shim uses `ResetForTest`; utilities tests use `NewTable` + `Table.Reset()`).

**Reproduce:** not reproduced — no failing alias/`Peek` test on `origin/master` (feature absent). The requester’s “probably fixed bug” matches missing alias unbind on incarnation teardown in master; vendor adds `unbindIncarnationLocked` on `endBusySlot`, `unmapLocked`, `installCloser`, `expire` (`expireDispose`), and `takeAll`.

**Outside facts:** bouncer vendor tree (change source); bouncer alias/peek tests (test matrix); `knowledge/devdocs/std_go_reclaim.md` (no alias/`Peek` language yet — devdocsimpact).

## Decisions

- PR scope is **only** the measured vendor delta: add `alias.go`, apply the `table.go` hunks; do **not** touch `opentyped.go`, other packages, or the bouncer repo.
- Treat every measured `table.go` hunk as in scope, including teardown unbind — that is the alias lifecycle fix, not an optional side quest.
- Implement tests in **utilities** `reclaim/` (new files such as `alias_test.go` / `peek_test.go` or focused tables beside existing patterns), covering the matrix below; do not land production code without them (requirement Desired).
- Rewrite the ad-hoc vendor banner on `alias.go` to normal package documentation voice (same precedent as upstreaming other bouncer vendor overrides); keep Yaegi-oriented comments on `Box` / `Published` / hook reentrancy.
- Live OpenSpec contract: fold into **`std_go_reclaim_value-lifecycle`** (alias weak refs, `Peek` non-bind semantics, incarnation-end alias clear); **`std_go_reclaim_context-lease`** only if propose finds holder/Peek interactions need a scenario there.
- Do not migrate `e2e/reclaimprobe` to alias/`Peek` in this change. Do not tag or release utilities in this run (requirement Out of scope for bouncer bump).
- **Public instance names** (`alias:lapi:…`): owned by the **caller** (bouncer plugin today); **Table** owns bind/unbind mechanics only — reuse `SetAlias` / `Watch` / `ClearPublisher` as vendored, no new naming layer in utilities.

## PR test matrix (implement — do not write in explore)

Port or re-express scenarios proven in bouncer `pkg/reclaim/zzz_alias_test.go`:

- Watch before `SetAlias` leaves typed empty; publish updates existing watchers.
- Independent LAPI vs AppSec groups: `ClearPublisher` on one group does not clear another.
- Second publisher on same alias rejected; first keeps alias.
- Grace `Close` clears watchers to empty.
- Dying incarnation unbind must not overwrite a replacement publisher’s value.
- Same-publisher rename clears old alias watchers.
- `changed` callback: no spurious fire on empty watch; one fire on publish; no duplicate on republish same pointer; late subscriber sees current; `ClearPublisher` fires clear.
- Watch subscriber dropped when `ctx` ends before later publish.
- `ClearPublisher` only drops owned aliases.

From `pkg/reclaim/zzz_peek_test.go`:

- Missing key → `ok=false`.
- Busy slot → `ok=false` without blocking.
- Awake peek does not bind (cancel sole holder → slot sleeps).
- Asleep peek does not accelerate grace or run `Close` early; after grace, `ok=false`.

**Additional utilities-only coverage for measured teardown wiring:**

- After `SetAlias`, unmap / grace expire / `Reset` (`takeAll`) / failed busy slot (`endBusySlot` / `installCloser` paths) each leave watchers on empty and allow a later `SetAlias`.
- `SetAlias` errors when key not mapped or slot not awake/asleep.
- `Watch` panics on nil `context` (documented API).

Follow existing reclaim test style: `NewTable(shortGrace)`, `waitBudget`, log/msg assertions where applicable; optional `-race` job unchanged.

## Open questions

- Q: Which OpenSpec change folder name and spec leaf take the alias / `Peek` delta?
  Rank: additive asked — Desired names faithful delta and extensive tests; lifecycle spec already owns reclaim behavior
  Decision: assumed — propose change slug `reclaim-alias` (or `2026-09-24-reclaim-alias` aligned with IssueKey); primary fold `std_go_reclaim_value-lifecycle`; usage update on `std_go_reclaim.md` in devdocsimpact.
  By: explore

- Q: Retain the vendor file header comment on `alias.go` verbatim?
  Rank: additive asked — requirement Unknowns
  Decision: assumed — replace ad-hoc “vendor override” banner with standard package/API comments; keep Yaegi constraint lines on exported types.
  By: explore

- Q: Post-merge tag / release for bouncer `go.mod`?
  Rank: additive incidental — requirement Out of scope and Unknowns; no criterion names a release in this repo
  Decision: assumed — out of this PR; note on card for consumer re-vendor after tag exists.
  By: explore

- Q: Add Yaegi harness cases for `SetAlias` / `Watch` / `Peek`?
  Rank: additive asked — requirement Desired “extensive test coverage”; `reclaim/yaegi_test.go` exists for reclaim
  Decision: assumed — compiled table tests are mandatory minimum; add Yaegi cases only where interpreter constraints matter (`Published` as `func(any)` argument, `Box` in `atomic.Value`), mirroring prior reclaim Yaegi patterns; skip under `-race` like existing interp tests.
  By: explore

- Q: Who owns the public alias string (instance name) vs the table’s bind state?
  Rank: additive asked — alias API separates publisher/group from table internals; bouncer already constructs names
  Decision: assumed — **caller** owns opaque alias/publisher/group strings; **Table** owns incarnation and aliasEntry graph; utilities PR does not embed Traefik instance naming.
  By: explore
