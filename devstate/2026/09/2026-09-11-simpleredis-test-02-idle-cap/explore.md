# Explore

## Concepts

SimpleRedis (`simpleredis/simpleredis.go`) is a stdlib pooled TCP RESP client. `borrow` reuses an idle socket younger than `idleTimeout` or dials; `release` appends a reusable socket to `idle` unless `sr.closed` or `len(sr.idle) >= maxIdleConns` (8). `Close` drains idle and marks closed; in-flight commands finish and their sockets are closed on `release`. Usage (`knowledge/devdocs/std_go_simpleredis.md`) already states the idle cap is eight after release and concurrent in-flight dials are not capped. Spec `std_go_simpleredis_tcp-session` still says concurrent commands SHALL not open more than eight connections — that SHALL is the copied-client total-cap claim this ticket must not implement (perf-01 is out of scope).

```
borrow ── idle hit ──────────────────────────────► do ──► release ──► idle append
   │                                              ▲              │
   ├── empty/stale ── 2nd closed? ── dial ────────┘              ├── closed or idle>=8 → close
   └── already closed → redis:unreachable (no dial, no release)
```

**Measured (DestBranch / this branch HEAD `0194b3f`, `go test ./simpleredis/ -count=1 -covermode=count`):** package pass, 86.1% statements. `release` 72.7%. Cover profile count **0** on `simpleredis.go:253.47,257.3` (3 stmts — `sr.closed || len(sr.idle) >= maxIdleConns` close path) and `simpleredis.go:236.12,238.3` (1 stmt — second `closed` check in `borrow`). `TestConcurrentCommandsStayWithinPool` and `TestCloseDrainsIdleAndDoesNotRepool` **PASS** in 0.00s. That is the claimed vacuity, not a failing test: eight goroutines cannot exceed eight accepts, and Get-after-Close returns `redis:unreachable` from `borrow:210-212` so `release` never runs. `startSlowRedis` / `simpleredis/bench_test.go` — not found (`openspec list --json` empty; no `bench_test.go` in tree). `startFakeRedis` accepts immediately (`simpleredis_test.go:15-50`). Existing delay pattern: `TestTimeoutOnReusedConnIsNotRetried` holds the reply with `time.Sleep`.

Live: `e2e/simpleredisprobe/plugin.go` one `SimpleRedis` per plugin, sequential verbs per HTTP request (already KEYS-declared Lua 5.1-safe EVAL). Pester `scripts/integration-tests.Tests.ps1:90-114` one `Invoke-WebRequest` per `/redis` and `/dragonfly`. Compose Redis `redis:7-alpine` and Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` share the project network; neither publishes 6379 to the host.

Identity (client address, user, tenant, Host, trust hop): this change does not set or reconstruct those facts. No owner question.

Devdocs: consumed `knowledge/devdocs/index.md` → `index_std_go.md` → `std_go_simpleredis.md`. Usage already matches the idle-vs-inflight split. Produce: write nothing.

Research: consumed `knowledge/research/index.md` → redis EVAL / dragonfly EVAL / dragonfly container image. No CLIENT LIST packet. Official Redis `CLIENT LIST` lists connections as LF-separated `property=value` lines ([redis.io CLIENT LIST](https://redis.io/docs/latest/commands/client-list/)). Dragonfly compatibility: CLIENT LIST fully supported; `TYPE`/`ID` filters rejected (syntax error) — use the no-argument form ([Dragonfly compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [dragonfly#7373](https://github.com/dragonflydb/dragonfly/issues/7373)). Did not write a research folder this phase (no Task delegate).

## Decisions

- Prove idle `len(idle) <= maxIdleConns` after overlap of **more than eight** commands, and that excess sockets are closed. Do not treat eight-goroutine / eight-accept as idle-cap proof. Do not add perf-01's total connection cap or wait queue.
- Unit tests own the deterministic idle-cap and in-flight-Close proofs against a hold/block fake in `simpleredis_test.go`. Live Pester on `/redis` and `/dragonfly` is the same idle-cap smoke against both engines, not a second implementation of the pool.
- Production pool constants and `release`/`borrow` behavior stay as they are except a nil-checked test hook for the second `closed` check if that hook is how we cover `:233-238`.
- Reuse the existing probe EVAL; do not add a new script. Do not export an idle-count getter; Pester reads `CLIENT LIST`.

## Open questions

- Q: How to drive genuine overlap on DestBranch without `startSlowRedis` / `bench_test.go`?
  Rank: additive asked — new test helper this change creates next to `startFakeRedis`; Desired "Unit-test the idle cap: more overlapping commands than `maxIdleConns`"
  Decision: assumed — add a same-file hold fake (accept or reply gated on a channel) modeled on `TestTimeoutOnReusedConnIsNotRetried`'s delayed listener. Start 16 Gets, release the hold after ≥16 accepts, then assert `len(idle) <= 8` and live fake sockets equal `len(idle)` (excess closed, not leaked). Do not add `bench_test.go`.
  By: propose

- Q: How does Pester observe idle socket count vs cap on live Redis and Dragonfly?
  Rank: additive asked — Desired "Extend compose + Pester `/redis` `/dragonfly`" and "live overlap against both engines must not leak idle sockets beyond the cap"
  Decision: assumed — fire 16 parallel `Invoke-WebRequest` at existing `/redis` and `/dragonfly` (plugin-internal overlap plus docker latency). After they finish, `docker compose exec redis redis-cli CLIENT LIST` and `docker compose exec redis redis-cli -h dragonfly CLIENT LIST` (bare command, no `TYPE`/`ID`). Count LF lines minus the listing client; remaining ≤ 8. No host port publish, no probe idle header, no `IdleCount` export. Existing ServeHTTP EVAL stays as-is (KEYS declared, Lua 5.1-safe).
  By: propose

- Q: Can `Close` between idle scan and `dial` be made deterministic without a test hook?
  Rank: additive asked — Affected "simpleredis.go only if a test-only hook is required"; Desired "Cover `borrow` `:233-238` (test hook or targeted unit test), or document that race as untested"
  Decision: assumed — add a nil-checked same-package hook after the idle-scan unlock and before the second `closed` check; the test sets it to `Close`. Nil in production is not a pool-behavior change. Do not leave `:236-238` documented-untested while that hook is in scope.
  By: propose

- Q: Does the spec SHALL "Concurrent commands SHALL not open more than eight connections" stay, given usage says in-flight dials are not capped?
  Rank: bounded asked — 3 live surfaces enumerated (searched `openspec/specs/std_go_simpleredis*`, `simpleredis/*_test.go`, `knowledge/devdocs/std_go_simpleredis.md` for "eight connections", "stay within the pool", `maxIdleConns`): `openspec/specs/std_go_simpleredis_tcp-session/spec.md` SHALL + scenario; `TestConcurrentCommandsStayWithinPool`; usage gotcha already correct. Archive copy not migrated. Desired + Tensions name matching the non-tautological idle-cap and in-flight-Close contracts.
  Decision: assumed — rewrite that SHALL/scenario to: after overlap of more than eight commands, idle ≤ 8; in-flight MAY exceed eight. Keep sequential-reuse. Add an in-flight-then-Close scenario. Repair `TestConcurrentCommandsStayWithinPool` so the goroutine count can violate the idle assertion (raise above 8, assert `len(idle) <= 8`). Keep Close-then-Get redial coverage; replace the vacuous final idle assertion with the in-flight-Close test that actually calls `release`.
  By: propose
