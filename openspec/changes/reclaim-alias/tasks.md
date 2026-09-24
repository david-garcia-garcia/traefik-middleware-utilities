## 1. Production delta (vendor-faithful)

- [x] 1.1 Add `reclaim/alias.go` from bouncer vendor; rewrite the file header to upstream package/API docs; keep Yaegi notes on `Box`, `Published`, and reentrancy
- [x] 1.2 Apply measured `reclaim/table.go` hunks: `aliases` map, slot reverse links, `Peek` + `State`, `unbindIncarnationLocked` on `endBusySlot`, `unmapLocked`, `installCloser` failure, `expireDispose`, and `takeAll`
- [x] 1.3 Confirm `reclaim/opentyped.go` is byte-identical to `origin/master`; do not edit other packages

## 2. Alias tests (`reclaim/alias_test.go` or focused tables)

Port scenarios from bouncer `pkg/reclaim/zzz_alias_test.go` using `NewTable(shortGrace)` and `Table.Reset()`:

- [x] 2.1 Watch-before-publish leaves empty; publish updates existing watchers
- [x] 2.2 Independent LAPI vs AppSec groups: `ClearPublisher` on one group does not clear the other
- [x] 2.3 Second publisher on same alias rejected; first keeps alias
- [x] 2.4 Grace `Close` (orphan + grace) clears alias watchers to empty
- [x] 2.5 Dying incarnation unbind does not overwrite a replacement publisher's value
- [x] 2.6 Same-publisher rename clears old alias watchers
- [x] 2.7 `changed` callback: no spurious fire on empty watch; one fire on publish; no duplicate on same pointer republish; late subscriber sees current; `ClearPublisher` fires clear
- [x] 2.8 Watch subscriber dropped when `ctx` ends before later publish
- [x] 2.9 `ClearPublisher` only drops aliases owned by that publisher/group
- [x] 2.10 `SetAlias` errors when key not mapped or slot not awake/asleep
- [x] 2.11 `Watch` panics on nil `context`

## 3. Teardown wiring tests (utilities-only)

- [x] 3.1 After `SetAlias`, unmap leaves watchers empty and allows later `SetAlias`
- [x] 3.2 After `SetAlias`, grace expire dispose leaves watchers empty
- [x] 3.3 After `SetAlias`, `Table.Reset()` leaves watchers empty and clears alias map
- [x] 3.4 Failed busy slot / `installCloser` failure path unbinds aliases (match vendor call sites)

## 4. Peek tests (`reclaim/peek_test.go`)

Port scenarios from bouncer `pkg/reclaim/zzz_peek_test.go`:

- [x] 4.1 Missing key → `ok=false`
- [x] 4.2 Busy slot → `ok=false` without blocking (timeout guard)
- [x] 4.3 Awake peek does not bind (cancel sole holder → slot sleeps)
- [x] 4.4 Asleep peek does not accelerate grace or run `Close` early; after grace, `ok=false`

## 5. Yaegi (optional, interpreter constraints only)

- [x] 5.1 Add `reclaim/yaegi_test.go` cases for `Watch` with `Published` delivered to `func(any)` and stable `Box` in `atomic.Value`, skip under `-race` like existing reclaim Yaegi tests

## 6. Specs and gates

- [x] 6.1 Keep change delta on `std_go_reclaim_value-lifecycle` aligned with apply
- [x] 6.2 `go test -count=1 ./reclaim/`
- [x] 6.3 Repo lint (`.golangci.yml`)
- [ ] 6.4 `Test-Integration.ps1 -Suite reclaim` when Docker available; `e2e/reclaimprobe` unchanged

## 7. Devdocs (devdocsimpact phase)

- [ ] 7.1 Update `knowledge/devdocs/std_go_reclaim.md` Language for alias, `Peek`, and weak vs strong refs (not in implement unless devdocsimpact runs same PR)
