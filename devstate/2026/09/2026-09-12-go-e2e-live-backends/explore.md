# Explore
IssueKey: 2026-09-12-go-e2e-live-backends

Verdict: in progress

## Concepts

Dest CI is three GitHub Actions jobs in `.github/workflows/ci.yml`:

```
  lint ──────── golangci-lint (no Redis)
  test ──────── go test ./...  + service containers Redis :6379 and Dragonfly :6380
                 LIVE_* env set so live_test.go does not skip
  integration ─ ./Test-Integration.ps1  (Pester + Traefik local plugins + compose)
```

The ticket names a **fourth suite**: compiled Go tests that need live Redis and Dragonfly. Dest already has that code (`simpleredis/live_test.go`, `windowcounter/live_test.go`, `tokenbucket/live_test.go`, plus `TestYaegiLive_*` on the two limiters). It rides **inside** `test`. Reproduced on this worktree: `go test ./simpleredis ./windowcounter ./tokenbucket ./reclaim` passed with no LIVE env (3.5s / 0.4s / 0.2s / 1.1s). `go test -v ./simpleredis -run TestLive_` skipped both live tests (`SIMPLEREDIS_LIVE_* unset`). Fake-TCP and Yaegi GOPATH tests do not need engines.

Two proof layers that are **not** the new suite:

| Suite | Owner | Needs live Redis/Dragonfly |
|---|---|---|
| Lint | `golangci-lint-action` + `.golangci.yml` | no |
| Unit Go | `*_test.go` fake TCP / in-process | no |
| Go live / e2e | `live_test.go` + `TestYaegiLive_*` | yes |
| Pester | `Test-Integration.ps1` | yes, but Traefik HTTP, not compiled client verbs |

Live SimpleRedis today is a thin slice: pool waiter `redis:unreachable`, `MSetEX` TTL + Get, past `MSetEXAt` miss, `CLIENT KILL` then Get. Get/Set/Del/MGet/Incr/Expire/Eval happy paths, AUTH, malformed RESP, LOADING retry stay on `startFakeRedis`. Window-counter and token-bucket live files already table-drive both engines; SimpleRedis Yaegi has no live counterpart.

`knowledge/devdocs/index.md` lists only `go` → `index_std_go.md` (reclaim, SimpleRedis, RESP decode, window counter, token bucket). No packet names the four suites. Per-library packets have one prove-with line each, not a catalog.

No identity reconstruction (client address, user, tenant, Host). Live tests take engine addresses from env; they do not invent a caller identity.

Research indexes (`ext_redis_*`, `ext_dragonfly_container-image`, `ext_dragonfly_msetex`) already cover the engines CI uses. No new research write.

```
  DestBranch `test` job
  ┌─────────────────────────────────────────┐
  │ redis:7-alpine :6379                   │
  │ dragonfly:v1.40.2 :6380                │
  │ go test ./...                          │
  │   fake TCP  ── always                  │
  │   live_*    ── skip unless LIVE env     │
  └─────────────────────────────────────────┘

  Ticket
  ┌──────────────┐  ┌─────────────────────────────┐
  │ test -short  │  │ e2e: same engines + LIVE env │
  │ no services  │  │ go test without -short        │
  └──────────────┘  │ every case × Redis+Dragonfly  │
                     └─────────────────────────────┘
```

## Decisions

**Split CI: unit `test` depends on nothing; new job owns live engines.** Desired 2 and 4. Move Redis/Dragonfly services and `*_LIVE_*` env off `test` onto a new job. Unit job runs `go test -short -timeout 2m -count=1 -v ./...` so `live_test.go` / `TestYaegiLive_*` skip even if a runner leaked env. E2e job starts the same images dest already pins (`redis:7-alpine`, `dragonfly:v1.40.2` published `:6380`), sets all six LIVE vars, runs `go test` **without** `-short`. Keep dest skip contract (`-short` or both addrs unset). Do not introduce `//go:build live` tags (specs already SHALL skip under `-short`).

**Name the new job `e2e` / display `Go E2E`.** Pester stays `integration` / `Integration Tests`. Ticket said “go integration”; colliding with the existing Pester job name would hide which suite failed.

**Every live case table-drives Redis and Dragonfly; one missing addr is a fail, not a silent skip.** Dest today: if only Redis is set, Dragonfly is `continue`d. Ticket: the suite MUST run on both. Skip the whole test only when `-short` or **both** unset (local `go test` without Docker). If exactly one addr is set, fail. CI e2e sets both.

**Maximize live coverage of engine-success paths; keep fake-TCP for peer-abuse.** Port onto `live_test.go` (both engines) every compiled scenario that talks Redis successfully today against `startFakeRedis`:

- SimpleRedis: Get hit/miss, Set+EX then Get, Del, MGet hits/misses/empty, Incr then IncrBy, Expire/ExpireAt `:1`, Eval integer, second Eval is EVALSHA, NOSCRIPT → EVAL (SCRIPT FLUSH then Eval), existing pool waiter / MSetEX / peer-close.
- Window counter: dest live already has N-then-deny, buffered two-client, sliding boundary, Peek-then-Take. Add live: Peek does not increment, Peek denied then slides, buffered Peek, expire-on-first-hit (INCR+EXPIRE).
- Token bucket: dest live already has burst, two-instance share, memory/Redis agree. Add live: refund when wait exceeds max delay.

Do **not** port: malformed/truncated RESP, garbage lengths, LOADING/TRYAGAIN retry, fake listener peer-close, MSetEX native-argv inspection, constructor validation, unreachable-host without a server. Those need a fake peer. AUTH/SELECT live proof already lives on DestBranch (`SIMPLEREDIS_LIVE_*_AUTH` requirepass siblings + SELECT 99); keep that in `{domain}_e2e_test.go`. Reclaim stays unit-only (no Redis client).

**Yaegi live is part of Go E2E.** `windowcounter/yaegi_test.go` and `tokenbucket/yaegi_test.go` already skip the same way as compiled live. Add `TestYaegiLive_RedisAndDragonfly` on SimpleRedis matching those siblings (compiled test owns start/skip; interpreted probe calls the verbs). Unit `-short` skips them; e2e runs them.

**Usage packet `knowledge/devdocs/std_go_test-suites.md`.** Catalog lint, unit `go test`, Go E2E, and Pester — what each represents, what it must not substitute for, skip/env rules. Allowlist is `std`/`go` (`knowledge/devdocs/domains.md`); do not add a `build` domain this run. Per-library packets keep a one-line prove-with that points at this catalog. README Tests names the four suites the same way. Propose/implement write the packet; explore did not invent Language beyond names dest already uses (Lint, Test, Integration Tests, live).

**Fold existing live-env SHALL onto the e2e job.** `openspec/specs/std_go_simpleredis_resp-commands`, `std_go_windowcounter_sync-flush`, `std_go_tokenbucket_lua-eval` say CI MUST set LIVE env and must not skip. After the split, that CI is the `e2e` job, not `test`. Pester remains not a substitute for compiled live.

## Open questions

- Q: Fourth GitHub Actions job vs keeping live files inside `test`?
  Rank: bounded asked — Desired 2 (`test` needs no backends) and Desired 4 (new Go e2e suite); existing call sites enumerated: `.github/workflows/ci.yml` (1 job), `README.md` Tests, `knowledge/devdocs/std_go_simpleredis.md` / `std_go_windowcounter.md` / `std_go_tokenbucket.md`, specs `std_go_simpleredis_resp-commands` / `std_go_windowcounter_sync-flush` / `std_go_tokenbucket_lua-eval`
  Decision: assumed — new job `e2e` (`Go E2E`) with dest’s Redis 7 and Dragonfly services + all `*_LIVE_*` env; `test` has no services and runs `go test -short`.
  By: explore

- Q: Build tags (`//go:build live`) vs dest env-skip + `-short`?
  Rank: additive asked — Desired 4 names a suite that needs live backends; dest skip contract is already the gate (`testing.Short` and unset env)
  Decision: assumed — keep env-skip + `-short`. Unit job passes `-short`. Do not add build tags.
  By: explore

- Q: How far does “cover as much as possible” go?
  Rank: additive asked — Desired 4 maximize coverage of behaviour that exists only on fake TCP; Out of scope forbids new product APIs and reclaim live Redis
  Decision: resolved — live every engine-success path listed in Decisions. DestBranch AUTH/SELECT live (SELECT 99, WRONGPASS on `SIMPLEREDIS_LIVE_*_AUTH`) folds into `pool_e2e_test.go`. Keep fake-TCP for malformed RESP, LOADING retry, constructor checks. Both engines on every live case.
  By: implement

- Q: Is Yaegi live (`TestYaegiLive_*`) in the e2e suite or the unit job?
  Rank: additive asked — Desired 4 Go tests that need live backends; windowcounter and tokenbucket already skip the same LIVE env; SimpleRedis Yaegi is fake-only
  Decision: assumed — Yaegi live belongs in Go E2E. Add SimpleRedis `TestYaegiLive_RedisAndDragonfly`. Unit `-short` skips them.
  By: explore

- Q: Packet name/fold for the suite catalog?
  Rank: additive asked — Desired 5 document suites in `knowledge/devdocs`; `index_std_go.md` has no catalog leaf; `domains.md` is only `std`/`go`
  Decision: assumed — `knowledge/devdocs/std_go_test-suites.md` (3 parts, existing allowlist). Do not add `build` this run.
  By: explore

- Q: Job display name “Go integration” vs existing Pester `Integration Tests`?
  Rank: additive asked — ticket names the new suite go integration; dest job `integration` is Pester
  Decision: assumed — GitHub job id `e2e`, name `Go E2E`. Leave Pester `integration` / `Integration Tests`.
  By: explore

- Q: If CI sets only one of Redis/Dragonfly, skip the other engine or fail?
  Rank: bounded asked — Desired 4 every test MUST run against both engines; dest `live_test.go` `continue`s an empty addr (3 files + 2 Yaegi live)
  Decision: assumed — skip only when `-short` or both unset; exactly one addr set → fail that test.
  By: explore

- Q: Live file names — one `live_test.go` vs domain-adjacent `_e2e_test.go`?
  Rank: additive asked — Desired 4 new e2e suite; human: tests for a domain stay next to that domain file (`limiter`, `limiter_test`, `limiter_e2e_test`, `limiter_yaegi_test`)
  Decision: resolved — Go `_test.go` suffix. `{domain}_test.go` unit, `{domain}_e2e_test.go` live, `{domain}_yaegi_test.go` interp, `{domain}_yaegi_e2e_test.go` Yaegi live. Shared helpers in `{package}_e2e_test.go` when they are not one domain.
  By: implement
