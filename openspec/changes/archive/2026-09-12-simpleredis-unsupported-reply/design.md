## Context

`readReply` returns `[][]byte` plus `clean`. `do` identity-compares `err == errIssue` before mapping other dirty errors through `ioError` (`redis:unreachable`). Dest already discards dirty sockets. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- A sixth exported token for well-framed unsupported types, passed through `do` like `errIssue`.
- `Eval` comment states the flat-array / `tostring` contract.
- Tests pin `readReply` error + `clean`, Eval redial after nested array, and the three-bulk token-bucket shape.

**Non-Goals:**
- Mapping `*-1` to `redis:miss`.
- Nested `Reply` type or flattening trees.
- `HELLO` / RESP3 session mode / pub/sub.
- Changing `tokenbucket` Lua.

## Decisions

1. **Exact sixth `Error()` token, not a type-byte suffix.** Callers match exact strings. Alternative: `fmt.Errorf("%w type=%c", errIssue, b)` — rejected; that breaks `Error() == redis:issue?` without a stable new token.

2. **`do` treats `errUnsupportedReply` like `errIssue`.** Identity compare (or a small protocol-sentinel helper) so dirty unsupported does not become `redis:unreachable`. Alternative: only change `readReply` — rejected; dest `do` would swallow the new error.

3. **Classify by well-framed vs malformed, not by verb.** Unknown type, nested `*`, `-` in array → unsupported. Empty line, missing CR, unparseable length, `*-1` → stay `redis:issue?`. Alternative: all ticket-table rows as unsupported — rejected; dest "Null array is not a miss" stays `redis:issue?`.

4. **`readReply` table is the contract test.** Call `readReply` with a `bufio.Reader` so `clean` is asserted. Command-level Eval nested uses a counting Accept fake (same pattern as `peerCloseFake.connections`). Alternative: Get-only table — insufficient; ticket names Eval redial.

5. **Keep `[][]byte`.** Document `tostring`. Alternative: recurse nested arrays — out of scope; no in-tree script needs a tree.

## Risks / Trade-offs

- [New dirty sentinel becomes `redis:unreachable` in `do`] → Mitigation: decision 2; test Eval nested Error() is unsupported, not unreachable.
- [Callers matching only `redis:issue?` miss nested Lua] → Mitigation: **BREAKING** token is the point; usage packet lists six strings.
- [HTTP-shaped still looks like unknown type] → Mitigation: same unsupported token as RESP3; still dirty.

## Migration Plan

Library Error() text change for those shapes. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
