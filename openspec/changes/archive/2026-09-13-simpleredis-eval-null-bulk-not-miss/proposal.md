## Why

Top-level RESP2 null bulk (`$-1`) is decoded as `redis:miss`. Lua `return false` / `return nil` is that same wire form, so `Eval` reports a key miss. Callers that treat `IsMiss` as “key absent” treat a successful Eval false as a Get miss.

## What Changes

- Decode treats a top-level `$` null as one nil slot with a nil error (`[][]byte{nil}, true, nil`), the same shape as an array null slot. `readBulk` may still use `errMiss` internally for `length < 0`.
- `Get` maps miss after a 1-slot reply when `values[0]==nil`. Empty bulk `$0` stays a non-nil empty slice (not a miss).
- `Eval` `return false` returns `[][]byte{nil}, nil`. `errors.Is(err, ErrMiss)` is false. Do not remap `ErrMiss` only inside `Eval`.
- MGET aligned nil slots stay as they are.
- Public `Eval(ctx, script, digest, keys, args)` stays. Tests pass `ScriptSHA1Hex`.
- Compiled tests land first (MUST fail on current dest under `go test -short ./simpleredis/`, no `//go:build bugrepro`), then the decode + Get mapping, then those tests plus Get miss, MGET nil slots, and `$0` not-miss.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-decode`: optional leading minus still parses; top-level `$-1` is a nil slot with nil error, not `redis:miss` for every verb. Get miss moves off this leaf.
- `std_go_simpleredis_resp-commands`: Get still maps a one-slot nil reply to `redis:miss`; empty bulk `$0` is not a miss; Eval top-level false/`$-1` is a nil slot with a nil error (not `ErrMiss`).

## Impact

- `simpleredis/resp.go` — `readReply` top-level `$` null bulk.
- `simpleredis/commands.go` — `Get` nil-slot → `errMiss`.
- `simpleredis/commands_eval_test.go` — Eval `$-1` is not miss (`startStaticRedis`, dest `Eval` signature).
- `simpleredis/commands_test.go` — keep `TestGetHitAndMiss`; add Get `$0` not-miss via `startStaticRedis` if missing.
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` and `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- Usage gotcha “Null bulk `$-1` is `redis:miss`” and decode packet `parseLen` minus wording catch up in implement / devdocsimpact.

Out of scope: handshake redial; Eval/MSetEX per-hop deadlines; remapping `ErrMiss` only inside `Eval`; changing MGET nil-slot behavior; SET storing Redis nil/false; changing the public Eval signature.
