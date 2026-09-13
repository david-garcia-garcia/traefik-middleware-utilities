## Context

`readBulk` on `length < 0` returns `errMiss`. Top-level `$` in `readReply` forwards that as `nil, true, errMiss`. Array `$` on the same `errMiss` `continue`s and leaves `values[i]==nil`. `Get` never sees a nil slot because `exec` already returned the miss. `Eval` is a passthrough of that decode. See proposal.md for why. Proceed policies: `devstate/explore.md`. Specs: decode yields a nil slot; Get still maps miss.

## Goals / Non-Goals

**Goals:**
- Decode owns “null bulk is a nil slot.” Get owns “one nil slot is `errMiss`.” Eval does not remap `ErrMiss`.
- Tests that reproduce land first and MUST fail on current dest under `go test -short ./simpleredis/` (no `//go:build bugrepro`). Then decode + Get mapping. Then those tests pass and Get miss, MGET nil slots, and `$0` not-miss still hold.
- Keep public `Eval(ctx, script, digest, keys, args)`. Adapt tests with `ScriptSHA1Hex`.

**Non-Goals:**
- Handshake redial; Eval/MSetEX per-hop deadlines.
- Remapping `ErrMiss` only inside `Eval`.
- Changing MGET aligned nil slots.
- Changing the public Eval signature.
- SET storing Redis nil/false.
- A second Lua `return nil` test besides wire `$-1`.

## Decisions

1. **Fix decode, map miss on Get.** Top-level `$` null returns `[][]byte{nil}, true, nil` (same as an array null slot). `Get` after a 1-slot reply: `values[0]==nil` → `errMiss`. Alternative: remap `ErrMiss` only inside `Eval` — rejected; that papers over the shared decode; MGET already has the slot shape.

2. **`readBulk` may still return `errMiss` for `length < 0`.** Array `$` already `continue`s on that sentinel. Alternative: a new null-bulk sentinel — rejected; one owner for “negative bulk length,” two call sites (top-level vs array).

3. **Empty bulk `$0` stays a non-nil empty slice.** `readBulk` `length==0` then `data[:0]` is already a hit. Get MUST NOT treat that as miss. Alternative: treat any empty/nil as miss — rejected; GET miss is `$-1`, not `$0`.

4. **Tests first, then the two-line change together.** Land compiled Eval `$-1` not-miss in `commands_eval_test.go` via `startStaticRedis` (`Eval(ctx, script, ScriptSHA1Hex(script), nil, nil)`). That test MUST fail on dest. Apply decode + Get in one step so `TestGetHitAndMiss` does not go red between them. Then confirm Eval, Get miss, MGET nil slots, and add Get `$0` via `startStaticRedis` if missing. Alternative: decode-only first — rejected; Get miss would break until the verb mapping lands.

5. **One compiled `$-1` Eval test is the Lua false/nil proof.** Official Lua→RESP2 lists `false` → null bulk; `return nil` shares that wire. Alternative: two script bodies — rejected; the contract is the wire form.

## Risks / Trade-offs

- [Decode lands without Get mapping] → Mitigation: decision 4; apply both in the same implement step; keep `TestGetHitAndMiss`.
- [Callers that treated Eval false as `IsMiss`] → Mitigation: that match is the bug; they MUST inspect the nil slot. In-tree Eval callers wrap `tostring` or return numbers.
- [`$0` Get silently becomes miss] → Mitigation: non-nil empty slice stays; add verb-level `$0` via `startStaticRedis` if dest has only `readBulk` empty-ok.

## Migration Plan

Library behavior change for Eval on top-level `$-1` only. Get miss and MGET slots stay. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
