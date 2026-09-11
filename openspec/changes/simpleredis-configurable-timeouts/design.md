## Context

Dest `simpleredis/simpleredis.go` stores `host` / `pass` / `database` only. Package constants `dialTimeout` (2s), `ioTimeout` (1s), `idleTimeout` (30s), `maxIdleConns` (8) feed `dial`, `do` (`SetDeadline` covering write+read), `borrow`, and `release`. `Init` is three strings and does not dial. `exec` does not retry `errTimeout`. Probe `e2e/simpleredisprobe` calls `Init(host, "", "")`. CI `test` job already starts Redis `:6379` and Dragonfly `:6380` and sets limiter live env vars; it does not set `SIMPLEREDIS_LIVE_*`. See proposal.md for why. Spec: `std_go_simpleredis_tcp-session`. Proceed policies: `devstate/explore.md`. BLPOP stall facts: `knowledge/research/ext_redis_blpop/`, `ext_dragonfly_blpop/`.

## Goals / Non-Goals

**Goals:**
- Caller-set dial / I/O / idle timeout and idle-list cap via `InitWithOptions` + plain `Options`, with today's constants as zero-value defaults.
- Fake proofs for configured knobs; live BLPOP I/O-timeout on both engines in CI.

**Non-Goals:**
- Wait-queue / live-socket semaphore (`poolSize` / `poolTimeout`) — sibling `simpleredis-live-pool-cap`.
- Idle-head reaper, pipelining, EVALSHA, encoding, MSETEX.
- Retries on timeout. Separate read vs write deadlines. Exported host/pass/database.
- Probe Config, Pester stall, public BLPOP, Yaegi `InitWithOptions` smoke.
- go-redis, miniredis, TLS, Unix sockets, functional options.

## Decisions

1. **`InitWithOptions(host, pass, database, Options)` plus `Init` → `InitWithOptions(..., Options{})`.** Plain exported `Options` of `time.Duration` and `int`. Alternative: extra `Init` args — rejected; would break every existing call. Alternative: exported timeout fields on `SimpleRedis` — rejected; would mix pool internals with settings.

2. **Resolve defaults at `InitWithOptions` into unexported session fields.** Copy `Options`, then if a duration is `<= 0` store the matching package constant; if `MaxIdleConns <= 0` store `8`. `dial` / `do` / `borrow` / `release` read those fields. Package constants remain the default table. Alternative: keep zeros on the struct and resolve at each use — rejected; package tests must see the stored duration `dial` uses. DTO `Options` sits next to `SimpleRedis` in `simpleredis.go`.

3. **One `SetDeadline(now+IoTimeout)` in `do`.** No `IoReadTimeout` / `IoWriteTimeout`. Alternative: split deadlines — rejected; Desired called them optional and one combined deadline enough.

4. **Idle cap stays the idle list.** `release` closes when `len(idle) >= maxIdleConns` using the session field. Do not add a live count or wait channel. `Options` has no `poolSize` / `poolTimeout`. Concurrent-eight wording in the spec stays as today.

5. **Fake: keep `TestIoTimeout` on `Init` defaults.** New never-reply fake with configured `IoTimeout` (tens of ms, assert `redis:timeout` well under 1s). Extend idle-timeout and idle-cap fakes with configured `IdleTimeout` / `MaxIdleConns`. Dial: hanging-SYN to `192.0.2.1` with short `DialTimeout` plus same-package assert that the stored dial duration is the configured value.

6. **`simpleredis/live_test.go` BLPOP stall.** Table-drive `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` like `windowcounter/live_test.go`. Skip on `testing.Short` or empty. Unique empty key per run (`t.Name()` plus a unique suffix). Same-package `exec` of `BLPOP`, that key, `"10"`. `IoTimeout` ~50ms. Do not export BLPOP. Do not `Eval` a BLPOP. Optional `waitLiveClient` via Get/Set/Incr (no Ping, no production Reset/Dump). CI `test` job sets `SIMPLEREDIS_LIVE_REDIS=127.0.0.1:6379` and `SIMPLEREDIS_LIVE_DRAGONFLY=127.0.0.1:6380` next to the limiter vars; do not add `-short`. README Tests names those vars. Alternative: Traefik/Pester stall — rejected; would 502 `/redis` for other Describes.

7. **Probe stays `Host` only.** `New` keeps `Init`. No compose or Pester change for this proof. Yaegi `clientprobe` keeps `Init`.

8. **Usage How to use after apply.** `knowledge/devdocs/std_go_simpleredis.md`: `InitWithOptions`; zero/negative = default; `Init` unchanged. Language stays. No Language write in this proposal.

## Risks / Trade-offs

- [BLPOP on shared CI Redis stalls other packages] → Mitigation: BLPOP blocks one connection; unique empty keys; do not EVAL or CLIENT PAUSE (`ext_redis_blpop`).
- [Live tests skip in CI] → Mitigation: set both `SIMPLEREDIS_LIVE_*` on the existing `test` job; do not pass `-short`.
- [`192.0.2.1` RSTs instead of blackholing] → Mitigation: also assert the stored dial duration; elapsed still well under 2s.
- [MaxIdleConns 0 would close every idle socket if treated as a literal cap] → Mitigation: `<= 0` means eight; callers cannot express idle cap zero on this type.
- [Yaegi cannot see unexported fields] → Mitigation: compiled same-package tests; no production Reset/Dump; no required Yaegi `InitWithOptions` smoke.

## Migration Plan

Additive method and type. Existing `Init` callers stay. Rollback is revert.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
