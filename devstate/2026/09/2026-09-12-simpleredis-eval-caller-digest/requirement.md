# Requirement
IssueKey: 2026-09-12-simpleredis-eval-caller-digest

## Problem
`SimpleRedis.Eval` SHA-1s the Lua body on every call through unexported `scriptSHA1Hex`. Callers that reuse a fixed script cannot compute the digest once at init and pass it in; they rehash on every Eval. The ask is caller control of that digest, plus an exported hash function.

## Current (code)
- `simpleredis/commands_eval.go:19-27` — `Eval(script string, keys []string, args []string)` calls `scriptSHA1Hex(script)` each time, sends `EVALSHA`, and on `NOSCRIPT` prefix sends `EVAL` with the body once.
- `simpleredis/commands_eval.go:30-34` — `scriptSHA1Hex` is unexported SHA-1 lowercase 40-char hex (`crypto/sha1` + `encoding/hex`). Comment: hash each call; no client digest table.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md:234` — public signature stays `Eval(script, keys, args)`; callers MUST NOT pass a digest; Eval SHALL hash the body on each call; MUST NOT export `EvalSha` or `ScriptLoad`; MUST NOT keep a digest table or mutex.
- `windowcounter/limiter.go:46-60` `New` stores `*simpleredis.SimpleRedis` only; `windowcounter/limiter.go:372` `Eval(flushScript, …)` on each Redis flush.
- `tokenbucket/redis.go:18-34` `NewRedis` stores the client only; `tokenbucket/redis.go:57` `Eval(allowScript, …)` on each `Allow`.
- `simpleredis/commands_msetex.go:64` — MSETEX Lua fallback `Eval(msetexFallbackScript, names, argv)` with no stored digest.
- `e2e/simpleredisprobe/plugin.go:102,186,197,223,276` — probe `Eval` of package script consts (and a hold script) with no stored digest.
- `simpleredis/yaegi_test.go:210,223,272` — interpreted `clientprobe` calls `Eval(script, keys, args)`.
- Exported `ScriptSHA1Hex` / `Eval` digest parameter — not found.

## Desired
1. `Eval` requires the SHA-1 hex from the caller (still needs the script body for the existing `NOSCRIPT` → `EVAL` fallback).
2. Export the hash that is today `scriptSHA1Hex` so callers compute Redis `sha1hex` themselves.
3. Callers that reuse a script (the consumption pattern in this tree) calculate and store that hex when the component is inited, and pass the stored digest into `Eval` so the client does not rehash every call.

## Affected
- `simpleredis/commands_eval.go` (signature + export).
- `simpleredis/commands_msetex.go` (fallback Eval).
- In-tree Eval callers: `windowcounter/limiter.go`, `tokenbucket/redis.go`, `e2e/simpleredisprobe/plugin.go`, `simpleredis` tests (`commands_eval_test.go`, `commands_eval_e2e_test.go`, `yaegi_test.go` `clientprobe`, `resp_test.go`, `bench_test.go`, `commands_exec_test.go`, `simpleredis_e2e_test.go`), `windowcounter` / `tokenbucket` tests that call `Eval`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (Eval signature and “MUST NOT pass a digest” / “hash each call”).

## Out of scope
- Exporting `EvalSha` or `ScriptLoad`, or sending `SCRIPT LOAD` at `Init`.
- A SimpleRedis-side digest table or mutex.
- Verifying the caller digest equals `scriptSHA1Hex(script)` before send.
- Changing the wire sequence (`EVALSHA` then `EVAL` on `NOSCRIPT`) or the `[][]byte` reply shape.
- Other audit-folder findings.

## Unknowns
- Parameter order of the new digest argument on `Eval`.
- Exported Go name (`scriptSHA1Hex` cannot be exported as-is; capital `ScriptSHA1Hex` is the language-legal export of that identifier).
- Whether a caller-supplied digest that does not match the body should fail, or keep today’s `NOSCRIPT` then `EVAL(body)` path (engine stores the real SHA-1 of the body).

## Tensions
- Live spec `std_go_simpleredis_resp-commands` says callers MUST NOT pass a digest and Eval SHALL hash every call. This ticket requires the opposite: caller-supplied digest, no rehash inside Eval.
- Dest comment on `Eval` says hashing ~470 B each call is cheaper than a mutexed map. The ticket still wants caller-held digests so reuse does not rehash.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md:8` says v1 MUST NOT use EVALSHA. Dest `tokenbucket` already admits via `simpleredis.Eval`, which sends EVALSHA. This ticket does not ask tokenbucket to stop using Eval; it asks that caller to hash `allowScript` at init.
