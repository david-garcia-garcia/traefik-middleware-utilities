# Requirement
IssueKey: 2026-09-13-simpleredis-bug-eval-null-bulk

## Problem
Top-level RESP2 null bulk (`$-1`) is decoded as `redis:miss`. Lua `return false` / `return nil` is that same wire form, so `Eval` reports a key miss. Callers that treat `IsMiss` as “key absent” treat a successful Eval false as a Get miss. `return {false}` is already an array nil slot with a nil error. SET cannot store Redis nil/false; this is only the EVAL reply.

## Current (code)
- `readBulk` on `length < 0` returns `errMiss`: `simpleredis/resp.go:188-189`.
- Top-level `$` in `readReply` forwards that as `nil, true, errMiss`: `simpleredis/resp.go:122-125`.
- Array `$` on `errMiss` `continue`s and leaves `values[i]==nil`: `simpleredis/resp.go:151-154`.
- `do` returns that error as a reusable reply: `simpleredis/resp.go:37-47`.
- `exec` forwards a non-retryable error (including `errMiss`) to the verb: `simpleredis/commands_exec.go:53-57`.
- `Get` returns `exec`’s error immediately and never inspects a nil slot: `simpleredis/commands.go:9-17`. Empty bulk `$0` is a non-nil empty slice from `readBulk` (`length==0` then `data[:0]`): `simpleredis/resp.go:188-203`.
- `Eval` returns `exec`’s `[][]byte` and error with no extra mapping: `simpleredis/commands_eval.go:26-32`. Public signature is `Eval(ctx, script, digest, keys, args)`: `simpleredis/commands_eval.go:26`.
- `MGet` already keeps aligned nil slots: `simpleredis/commands.go:20-37`; `simpleredis/commands_test.go:43-56`.
- Get miss today is `TestGetHitAndMiss` via the fake store (missing key → `$-1`): `simpleredis/commands_test.go:8-22`. No compiled test asserts Eval `$-1` is not `ErrMiss`. `startStaticRedis` exists: `simpleredis/fake_redis_test.go:661-688`.
- Decode spec: optional leading minus so `$-1` remains `redis:miss`; scenario “server replies `$-1` → Get returns `redis:miss`”: `openspec/specs/std_go_simpleredis_resp-decode/spec.md:45-49`. Commands spec: Get null bulk SHALL be `redis:miss`; Eval return is `[][]byte` or a `-` error, with no Eval-false scenario: `openspec/specs/std_go_simpleredis_resp-commands/spec.md:8`, `:245-246`.
- Lua `false` → RESP2 null bulk (research; `return nil` not listed): `knowledge/research/ext_redis_eval/notes.md`. GET miss is `$-1`, not `$0`: `knowledge/research/ext_redis_resp_bulk-string/notes.md`.

## Desired
- Null bulk is a nil slot, not `ErrMiss` from decode.
- `readBulk` may still use `errMiss` internally for `length < 0`.
- `readReply` top-level `$` null: return `[][]byte{nil}, true, nil` (same as an array null slot).
- `Get`: after a 1-slot reply, `values[0]==nil` → `errMiss`. Empty bulk `$0` stays a non-nil empty slice (not a miss).
- Eval `return false` → `[][]byte{nil}, nil`. `errors.Is(err, ErrMiss)` must be false.
- Do not remap `ErrMiss` only inside `Eval`. MGET aligned nil slots stay as they are.
- Update specs if decode currently says `$-1` remains `redis:miss` for every verb — Get still maps miss; decode yields a nil slot.
- Implement order (human override; required): (1) land compiled tests that reproduce and MUST fail on current dest (`go test -short ./simpleredis/`, no `//go:build bugrepro`); reuse `startStaticRedis`; adapt the Eval repro to dest `Eval(ctx, script, digest, keys, args)` in `commands_eval_test.go` or `resp_test.go`; (2) apply the agreed how; (3) confirm those tests pass AND Get miss still works AND MGET nil slots AND `$0` empty bulk is not miss. Add Get `$-1` still `ErrMiss` and `$0` Get not miss if they are missing; keep existing tests passing.

## Affected
- `simpleredis/resp.go` (`readReply` top-level `$`)
- `simpleredis/commands.go` (`Get` nil-slot → `errMiss`)
- `simpleredis/commands_eval_test.go` and/or `simpleredis/resp_test.go` (Eval `$-1` not miss; Get `$-1` / `$0` if missing)
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (decode `$-1` is a nil slot, not miss for every verb)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` only if Eval false / nil-slot reply needs a scenario (Get miss stays)

## Out of scope
- Handshake redial
- Eval/MSetEX per-hop deadlines
- Remapping `ErrMiss` only inside `Eval`
- Changing MGET nil-slot behavior
- SET storing Redis nil/false

## Unknowns
- Official Lua-to-RESP2 table lists `false` → null bulk; ticket also names `return nil`. Wire `$-1` is the agreed how either way.
- Whether dest already has a `$0` Get test besides `readBulk` empty-ok (`simpleredis/resp_test.go:310-321`); implement adds one if missing.

## Tensions
- Example repro calls `Eval(ctx, "return false", nil, nil)`; dest `Eval` requires `digest` as the third argument (`simpleredis/commands_eval.go:26`). Adapt; do not change the public signature.
- Decode spec wording “`$-1` remains `redis:miss`” vs agreed how: decode yields a nil slot; Get still maps miss.
- Eval comment that Lua authors wrap each slot with `tostring` (`simpleredis/commands_eval.go:25`) vs allowing top-level false as a nil slot. Ticket is the EVAL reply only; keep tostring for multi-slot scripts unless a later phase reshapes it.
