## Why

`readReply` maps nested Lua tables, error-in-array, and unknown/RESP3 type bytes to `redis:issue?` and closes the socket. That same token covers missing CR and unparseable lengths, so a script author cannot tell a supported-shape miss from a decoder limit. `Eval` does not document that replies must be flat arrays of bulk strings or integers (`tostring`).

## What Changes

- Export `redis:unsupported-reply` for well-framed replies this decoder does not decode: unknown type byte (including HTTP-shaped and RESP3), nested array element, `-` inside an array. Keep `clean == false` (socket discarded, not retried).
- Keep malformed framing as `redis:issue?` (empty line, missing CR, unparseable length, empty element line). Keep `*-1` as `redis:issue?`, not miss, idle empty.
- **BREAKING** for callers matching `Error() == redis:issue?` on those unsupported shapes; they MUST match `redis:unsupported-reply`.
- Document on `Eval` that the return is a flat array of bulk strings or integers; Lua authors wrap slots with `tostring`. Do not change `[][]byte`. Do not recurse nested arrays.
- Unit table over `readReply` asserting error + `clean`. Eval nested-array fake that shows a redial (accept count). Guard that three bulk strings (token-bucket shape) still parse.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: sixth exported error string `redis:unsupported-reply`; unknown type, nested array, and `-` array element use that token (still not pooled); `Eval` documents the flat-array / `tostring` contract; `*-1` stays `redis:issue?`.

## Impact

- `simpleredis/simpleredis.go` — new exported const and `errUnsupportedReply`.
- `simpleredis/resp.go` — `readReply` defaults and array-element default; `do` MUST pass the new sentinel through (not `ioError`).
- `simpleredis/commands_eval.go` — `Eval` comment.
- `simpleredis/resp_test.go` — split malformed vs unsupported; `readReply` table; Eval redial; three-bulk guard.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` after archive.
- `knowledge/devdocs/std_go_simpleredis.md` — sixth token and Eval `tostring` rule.
