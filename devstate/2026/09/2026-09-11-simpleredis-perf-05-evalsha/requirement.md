# Requirement
IssueKey: 2026-09-11-simpleredis-perf-05-evalsha

## Problem
`SimpleRedis.Eval` ships the full Lua body on every call (`EVAL` + `[]byte(script)`). Rate-limit hot paths (`tokenbucket`, `windowcounter` flush) pay that wire cost on every decision. Dest has no digest cache, no `EVALSHA`, and no `NOSCRIPT` fallback.

## Current (code)
- `simpleredis/simpleredis.go` `Eval` — always `EVAL` + full script + decimal `numkeys` + keys + args; public signature `Eval(script string, keys []string, args []string) ([][]byte, error)`.
- `simpleredis/simpleredis.go` `replyError` — maps only AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`); any other `-` payload (including `NOSCRIPT`) is returned as `errors.New(text)` to the caller.
- `simpleredis/simpleredis.go` `SimpleRedis` — one `mu` for the idle pool; no script-to-digest map.
- `simpleredis/simpleredis.go` `exec` — retries once on a dead reused idle socket; does not special-case `NOSCRIPT`.
- `simpleredis/simpleredis_test.go` `fakeRedis` — `EVAL` only; unknown commands (`EVALSHA`) reply `+OK`; `lastEval` records EVAL argv; no NOSCRIPT path.
- `simpleredis/simpleredis_test.go` `TestEvalArgvAndIntegerReply`, `TestEvalEmptyKeys`, `TestEvalMixedArrayReply`, `TestEvalNestedArrayIsIssue` — compiled EVAL argv/reply only.
- `simpleredis/yaegi_test.go` `TestYaegi_IncrAndEval` — interpreted `Eval` against the compiled fake (stdlib.Symbols, `useunsafe` false); no EVALSHA/NOSCRIPT fallback.
- `e2e/simpleredisprobe/plugin.go` — one `Eval` per request; headers include `X-SimpleRedis-Eval`; does not assert digest vs body.
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`; `/redis` and `/dragonfly` whoami routes.
- `scripts/integration-tests.Tests.ps1` — Pester asserts `X-SimpleRedis-Eval` == `3` on `/redis` and `/dragonfly`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — `Eval` SHALL send `EVAL`; **EVALSHA and SCRIPT LOAD MUST NOT be added**.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — admit via `simpleredis.Eval`; **v1 MUST NOT use EVALSHA**.
- `knowledge/devdocs/std_go_simpleredis.md` — documents `Eval`; no EVALSHA/NOSCRIPT.
- `knowledge/research/ext_redis_eval/notes.md` — Redis EVAL shape, Lua 5.1, KEYS/ARGV.
- `knowledge/research/ext_dragonfly_eval/notes.md` — Dragonfly EVALSHA exists; Lua 5.4.4; undeclared keys forbidden; no `table.maxn`.
- `knowledge/research/ext_traefik_ratelimiter_token-bucket/.sources/script.go.md` — go-redis `Run`: EVALSHA then EVAL on `NOSCRIPT` (v1 note said do not copy).
- No dedicated `knowledge/research/` folder for Redis/Dragonfly `EVALSHA` / `NOSCRIPT` wire text.

## Desired
- Keep the public `Eval` API unchanged. Callers still pass the script body.
- Cache SHA-1 of each distinct script on the client (`crypto/sha1`, stdlib; mutex-guarded). No `SCRIPT LOAD` at `Init`.
- Send `EVALSHA <digest> <numkeys> …`. On an error whose text starts with `NOSCRIPT`, fall back once to `EVAL` with the full body, then keep using the digest. Do not surface that `NOSCRIPT` to the caller as the command result.
- Fake tests: later calls send `EVALSHA` + digest not the body; first `EVALSHA` can get `-NOSCRIPT No matching script`, then `EVAL` succeeds, then `EVALSHA` again; two scripts get two digests.
- Yaegi interp coverage of the NOSCRIPT fallback (`simpleredis/yaegi_test.go`, stdlib.Symbols, no Traefik).
- Live proof on both engines: extend compose + Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe` so EVALSHA + NOSCRIPT fallback is proven against Redis (Lua 5.1) and Dragonfly (Lua 5.4). Scripts must list keys in KEYS and must not use `table.maxn`.
- Update `std_go_simpleredis_resp-commands` so EVALSHA-inside-Eval is required, not forbidden. Public signature stays the same.

## Affected
- `simpleredis/simpleredis.go`, `simpleredis/simpleredis_test.go`, `simpleredis/yaegi_test.go`
- `e2e/simpleredisprobe/plugin.go`
- `scripts/integration-tests.Tests.ps1` (and compose only if probe/routes need a new signal)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (+ change folder in propose)
- `knowledge/devdocs/std_go_simpleredis.md` (later phases)
- Callers `tokenbucket/` and `windowcounter/` stay on `Eval` (no signature change)

## Out of scope
- Pipelining (perf-04), MSETEX (feat-01), `unsafe` zero-copy (perf-08).
- Eager `SCRIPT LOAD` at `Init`.
- New exported method (`EvalSha`, `ScriptLoad`). Changing `Eval`'s signature.
- `go-redis`, miniredis, TLS, Unix sockets.
- Pool/timeout/retry policy except recognising `NOSCRIPT` as the Eval fallback signal.
- Rewriting token-bucket or window-counter Lua; those already list KEYS and avoid `table.maxn`.

## Unknowns
- Exact `NOSCRIPT` error prefix on Dragonfly v1.40.2 vs Redis 7 (finding assumes text starts with `NOSCRIPT`; no dedicated research packet this phase — worker did not spawn research).
- How the probe/Pester request can prove EVALSHA vs a one-shot EVAL on a live engine (one Eval per GET today; SCRIPT FLUSH / two-hit header design TBD in explore).
- Whether `std_go_tokenbucket_lua-eval` "v1 MUST NOT use EVALSHA" is only a caller-surface rule (still true if they call `Eval`) or must be rewritten when Eval internally uses EVALSHA.

## Tensions
- Live spec `std_go_simpleredis_resp-commands` forbids EVALSHA; this ticket requires EVALSHA inside `Eval`. Follow the ticket; propose updates that spec.
- `std_go_tokenbucket_lua-eval` says v1 MUST NOT use EVALSHA; this ticket keeps the public Eval API and still wants EVALSHA on the wire. Tokenbucket/windowcounter call sites stay `Eval`.
- Finding's proof plan is fake-server + Yaegi; conductor HARD REQUIREMENT also requires live Redis and Dragonfly via compose/Pester/probe. Take both.
- Index suggested doing perf-05 with pipelining and MSETEX; this ticket is perf-05 only.
