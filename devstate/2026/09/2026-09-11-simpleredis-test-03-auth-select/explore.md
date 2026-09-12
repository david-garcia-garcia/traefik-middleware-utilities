# Explore
IssueKey: 2026-09-11-simpleredis-test-03-auth-select
Verdict: in progress
Qualify: qualified-with-gaps (continue)

## Concepts

**Handshake `dial`**: `Init` stores `host`, `pass`, `database` and does not connect. The first command `borrow`s; empty idle → `dial`. `dial` opens TCP, then if `pass != ""` sends `AUTH`, then if `database != ""` sends `SELECT`. Either command error: `conn.close()`, return the error, do not append to `idle`. AUTH is always first when both are set (`simpleredis/simpleredis.go`).

**`exec` does not retry a failed handshake**: `exec` retries only when a *reused idle* socket is dead. `borrow` returns `reused=false` on a new `dial`. A handshake error is `borrow`'s error and returns immediately (`:184-186`). One failed AUTH/SELECT is one error, not a redial loop.

**`replyError`**: `-` payload after the dash. Prefixes `NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH` → `redis:noauth`. Anything else (including `ERR DB index is out of range` and Redis 7.4 nopass AUTH text) is `errors.New` of that text.

**In-process fake**: `startFakeRedis` always `+OK` on AUTH/SELECT. `startStaticRedis` uses one canned reply for every command, so AUTH-then-SELECT cannot succeed then fail. No close counter; `serve` `defer conn.Close()` is the fake's own close.

**Live engines (measured 2026-09-11)**: dest images `redis:7-alpine` (7.4.10) and `dragonfly:v1.40.2`. Both: `SELECT 99` → `ERR DB index is out of range` (16 DBs). Both with `--requirepass secret`: unauth `PING` → `NOAUTH Authentication required.`; `AUTH wrong` → `WRONGPASS invalid username-password pair or user is disabled.`. **Differ:** Dragonfly without requirepass: `AUTH wrong` → `OK`. Redis without requirepass: `ERR AUTH <password> called without any password configured for the default user. Are you sure your configuration is correct?` (not `ERR Client sent AUTH…`, not `redis:noauth`).

**Coverage gap (reproduced)**: `go test ./simpleredis/` 86.1%; `dial` 71.4%. Cover profile count 0 on blocks `277.85,280.4` (AUTH error close/return) and `283.91,286.4` (SELECT error close/return). Success AUTH/SELECT bodies are count 1 (`TestAuthAndSelectOncePerDial`). `TestRejectedAuthIsReturned` Inits `pass=""` so AUTH is skipped; `-NOAUTH` answers GET.

**Live harness**: compose `redis` / `dragonfly` have no password. Probe `Config` is `Host` only; `Init(host, "", "")`. Pester `/redis` `/dragonfly` success + Eval KEYS snippet. CI `test` job already publishes unpassworded Redis `6379` and Dragonfly `6380` for windowcounter/tokenbucket live tests. Integration job is compose + Pester.

```
Init(host, pass, db)     no TCP
        │
 first Get/Set/…
        ▼
    borrow → dial
        │
        ├─ pass!="" ── AUTH ─ err? close, return (uncovered)
        │                 OK
        ├─ db!="" ── SELECT ─ err? close, return (uncovered)
        │                 OK
        └─ return conn → pool after command
```

## Decisions

- Tests MUST run against both Redis and Dragonfly (conductor + requirement Desired).
- Handshake AUTH/SELECT failure branches need the configurable fake **and** live proof on both engines for the cases each engine supports (wrong password / bad DB index).
- Redis and Dragonfly differ (Dragonfly nopass AUTH succeeds; Redis 7.4 nopass AUTH is a non-`redis:noauth` ERR). Still run every live case each engine supports; do not skip Dragonfly because it differs.
- Dest compose + Pester **can** take a passworded service: extra sibling containers and probe `Password`/`Database` labels; keep existing no-password `/redis` and `/dragonfly`.
- Any Eval the harness still sends stays the existing Lua 5.1-safe KEYS-declared Kong snippet. No new Lua (Out of scope).
- Fake: optional AUTH/SELECT reply fields on `fakeRedis` defaulting to `+OK` (existing success tests unchanged). Handshake-failure tests set those replies. AUTH prefix cases: `Init(addr, "wrong-password", "")` for `NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`. SELECT fail: `Init(addr, "secret", "99")` so AUTH runs first (`+OK`) then SELECT `-ERR DB index is out of range`. Keep `TestRejectedAuthIsReturned` as command-reply mapping.
- Peer saw close: increment a hangup counter when the fake `serve` read loop exits (client `conn.close()` → EOF). Assert `len(sr.idle)==0` and that counter. One accept (`connections()==1`) proves no `exec` redial storm.
- Live SELECT 99: both engines on dest's unpassworded compose **and** CI service ports. Go tests skip-if-unset like `windowcounter/live_test.go`, env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (same 6379/6380 as existing CI services). Pester: extra whoami routes with `database=99` against existing `redis`/`dragonfly`.
- Live wrong password: **requires** `--requirepass` (Dragonfly nopass AUTH is OK). Add compose siblings `redis-auth` / `dragonfly-auth` (`redis-server --requirepass …`, Dragonfly `--requirepass=…`, `ulimits memlock: -1` on Dragonfly). Extra whoami routes with `password=wrong`. Pester expects 502 body `redis:noauth`. Do not put requirepass on the existing success services.
- CI `test` job: add passworded sibling services + `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` so Go WRONGPASS tests run, not skip. Do not add requirepass to the existing windowcounter/tokenbucket Redis/Dragonfly.
- Probe `Config`: add `Password` and `Database` (empty default). `New` passes them to `Init`. Success labels stay host-only. Traefik 502 already returns `err.Error()`.
- Spec: `std_go_simpleredis_tcp-session` gains AUTH/SELECT **failure** scenarios (close, not pooled, mapped error, no retry). `std_go_simpleredis_resp-commands` keeps the existing no-password empty-database SHALL for `/redis` and `/dragonfly`; add scenarios for the new failure routes. Do not rewrite the success SHALL into a passworded compose.
- Production `dial` / `replyError` / pool stay unchanged unless a new test proves them wrong (Out of scope). Redis 7.4 nopass AUTH text is **not** a proof that `replyError` is wrong for this ticket.
- Coverage proof of blocks `277.85,280.4` and `283.91,286.4` is the in-process fake (`go test`). Live engines prove dest image behavior; they do not have to drive those cover counters.

## Open questions

- Q: How does the fake observe that the client closed the socket?
  Rank: additive asked — new counter on the test fake this change creates; Desired "peer saw close"
  Decision: assumed — count serve-loop exit (EOF after `pooledConn.close`); assert it plus empty `idle`.
  By: explore

- Q: How to add a passworded live service without breaking no-password `/redis` and `/dragonfly`?
  Rank: additive asked — new compose services and probe fields; Desired "extend compose + Pester if dest's harness can take a passworded service"; existing labels stay host-only
  Decision: assumed — sibling `redis-auth` / `dragonfly-auth` plus probe `Password`/`Database`; existing services and success Pester unchanged.
  By: explore

- Q: Which live AUTH/SELECT cases does each dest engine support?
  Rank: additive asked — Desired live proof per engine; Unknowns on requirement.md
  Decision: resolved — both engines: `SELECT 99` → `ERR DB index is out of range`; both with requirepass: `AUTH wrong` → `WRONGPASS` → `redis:noauth`. Dragonfly without requirepass: AUTH any password `OK` (not a fail case). Redis 7.4 nopass AUTH is not `redis:noauth`. `NOAUTH`/`NOPERM`/`ERR Client sent AUTH` are fake-only handshake AUTH replies on these images.
  By: explore

- Q: Should Go live tests use extra CI requirepass services or only Pester?
  Rank: additive asked — Desired tests against both engines; dest already hosts unpassworded engines in `.github/workflows/ci.yml` (2 services, WINDOWCOUNTER_/TOKENBUCKET_ live env)
  Decision: assumed — both: Pester on compose siblings, and extra CI passworded services so `go test` WRONGPASS is not skip-only. SELECT 99 uses the existing unpassworded CI services.
  By: explore

- Q: Should `replyError` also map Redis 7.4 `ERR AUTH <password> called without any password configured…` to `redis:noauth`?
  Rank: additive incidental — new prefix on a helper this change did not create; Out of scope "Changing dial / replyError / pool unless a test proves it is wrong"
  Decision: resolved — do not change `replyError`. Live Redis nopass AUTH asserts the raw ERR text if tested at all; wrong-password live case is requirepass `WRONGPASS`.
  By: explore

- Q: Rename `TestRejectedAuthIsReturned` now that it is not handshake coverage?
  Rank: additive incidental — existing test name; Tensions say keep command-reply tests and add handshake tests
  Decision: assumed — keep the name; add new handshake test names that say AUTH/SELECT failure.
  By: explore
