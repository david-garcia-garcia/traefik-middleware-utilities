# Requirement
IssueKey: 2026-09-12-simpleredis-risk-04-non-basic-resp2-replies-rejected

## Problem
`readReply` accepts only basic RESP2 (`+`, `:`, `-`, `$`, and a flat `*` of `$` / `:` / `+`). Any other shape, including a legal RESP2 null array, a nested Lua table, an error inside an array, and RESP3 type bytes, returns `redis:issue?` and closes the socket. The same sentinel covers malformed framing, so a Lua author who returns a nested table sees an unexplained failed command plus pool churn. The ticket asks to pick one scope, not all three: document and distinguish the failure; optionally treat `*-1` as a reusable miss; or support nested arrays.

## Current (code)
- `simpleredis/resp.go` `readReply` — `+`/`:` copy payload `clean true`; `-` via `replyError` `clean true`; `$` bulk or `errMiss` `clean true`; `*` parses count then each element `$` / `:` / `+` only; `default` and empty line return `errIssue` `clean false`.
- `simpleredis/resp.go` `*` branch — `!ok || count < 0` (including `*-1`) returns `errIssue` `clean false`. Comment: `*-1` is a legal RESP2 nil array (BLPOP timeout, EXEC abort); this client has no verb that receives it, so it is `redis:issue?` not `redis:miss`.
- `simpleredis/resp.go` array element `default` — nested `*` or `-` (Lua `{err=...}` in a table) returns `errIssue` `clean false`.
- `simpleredis/resp.go` `do` — when `readReply` errors and `!clean`, returns `errIssue` (or `ioError`); `simpleredis/pool.go` `release` closes the socket when `reusable` is false.
- `simpleredis/commands_exec.go` `shouldRetry` — retries `errUnreachable` and a short list of Redis `-` prefixes; `errIssue` is not among them, so the command fails once.
- `simpleredis/commands_eval.go` `Eval` — comment describes EVALSHA / NOSCRIPT only; return type `[][]byte`; no mention of flat arrays or `tostring`.
- `simpleredis/simpleredis.go` — exported match strings are `redis:unreachable`, `redis:miss`, `redis:timeout`, `redis:noauth`, `redis:issue?`. No `redis:unsupported-reply`.
- `simpleredis/commands.go` `parseIntegerReply` — requires exactly one `[][]byte` slot; arity/garbage is `errIssue` without dirtying a clean decode.
- `tokenbucket/lua.go` — `return {tostring(true), tostring(wait_duration), tostring(tokens)}`.
- `simpleredis/resp_test.go` `TestEvalMixedArrayReply` — fake `*3` of `$` / `:` / `+` succeeds. `TestMalformedReplyIsIssueAndNotPooled` — `*-1`, nested `*1\r\n*0\r\n`, unknown type, HTTP-shaped, bad element type: all `redis:issue?`, idle 0. Cases use `Get`, not `Eval`. No table asserts `readReply` `clean` directly. No redial/accept-count proof for nested Eval. No isolated three-bulk-string pin of the tokenbucket shape.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — malformed RESP (unknown type, `*` count `< 0`, nested array, element not `$`/`:`/`+`) SHALL be `redis:issue?` and MUST NOT re-enter idle. Scenario "Null array is not a miss": `*-1` → `redis:issue?`, not `redis:miss`, idle empty. Exported error strings SHALL be those five tokens only.
- `knowledge/devdocs/std_go_simpleredis.md` — `*-1` is `redis:issue?`; `$-1` is `redis:miss`. Eval how-to does not state the flat-array / `tostring` constraint.
- `knowledge/research/ext_redis_resp_null-array/notes.md` — official `*-1` vs null bulk vs empty array; SimpleRedis has no BLPOP/MULTI/EXEC; Lua `false` is null bulk not null array.
- `knowledge/research/ext_redis_eval/notes.md` — Lua indexed table → RESP2 array; nested tables → nested arrays; `{ err = "..." }` → `-` error reply.
- HELLO / RESP3 handshake: not found in `simpleredis/` command send path (`writeCommand` encodes bulk-string argv only).

## Desired
Pick one scope (ticket: do not do all).

1. Minimum (recommended): document on `Eval` that replies are flat arrays of bulk strings or integers (Lua authors wrap with `tostring`); make an unsupported type byte distinguishable from malformed framing (`redis:unsupported-reply` or wrap `errIssue` with the type byte). Keep dirty-socket on unsupported/malformed so there is no desync.
2. Optional cheap: in the `*` branch, map `*-1` to `errMiss` `clean true` so the socket stays reusable.
3. Larger: recurse nested arrays only if a planned script needs a tree. That requires a reply shape other than `[][]byte` (or a flattening contract) and ripples into `parseIntegerReply` and callers.

Prove with a `readReply` table for the measured rows (specific error + `clean`); an Eval nested-array fake that shows socket close and redial; if `*-1` becomes a miss, same-socket follow-up with no extra accept; a guard that the three-bulk-string tokenbucket shape still parses.

## Affected
- `simpleredis/resp.go` (`readReply` `*` / `default`)
- `simpleredis/commands_eval.go` (`Eval` comment, maybe return/error)
- `simpleredis/simpleredis.go` if a new exported sentinel is added
- `simpleredis/resp_test.go` (and possibly Eval/pool tests)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` if `*-1` or the error-string set moves
- `knowledge/devdocs/std_go_simpleredis.md` (Eval constraint; `*-1` gotcha if it moves)

## Out of scope
- Other `simpleredisfixes2/` findings (including bug-04 bulk trailer).
- Sending `HELLO`, RESP3 session mode, pub/sub, or client-side caching.
- Implementing nested-array trees unless explore picks that scope.
- Changing `tokenbucket` Lua unless a chosen scope requires it.
- Planned limiter scripts (`handoff-kong-window-limiter`, `handoff-leaky-bucket`) except as motivation that Eval authors will copy the convention.

## Unknowns
- Which of the three scopes this run takes (ticket forbids doing all three).
- Whether a new exported sentinel is allowed: dest spec pins exactly five `Error()` strings.
- Whether reversing the dest `*-1` → `redis:issue?` spec (and the usage gotcha) is in scope, given no current verb receives a null array.
- Whether any planned limiter script actually needs a nested table (gates the larger scope).
- How to surface the type byte if wrapping `errIssue` without a new public string.

## Tensions
- Ticket optional: `*-1` as `redis:miss` `clean true`. Dest spec, usage packet, `readReply` comment, and `TestMalformedReplyIsIssueAndNotPooled` pin `*-1` as `redis:issue?` not miss, idle empty, because this client has no verb that receives it.
- Ticket recommended: new sentinel `redis:unsupported-reply`. Dest spec: callers match exactly the five existing strings; `redis:issue?` already covers unknown type, nested array, and `*-1`.
- Ticket larger: nested arrays / `Reply` type. Dest `Eval` and `do` return `[][]byte`; incr-eval already chose nested `*`/`-` → `redis:issue?` and tests that.
- Ticket: RESP3 rows are lower risk because no `HELLO`. Dest send path has no `HELLO` (not found). Explicit `>` reject is future-only.
- Ticket: no current test isolates the tokenbucket three-bulk shape. Dest has `TestEvalMixedArrayReply` (mixed bulk / integer / status) but not three bulks, and malformed cases go through `Get` not `Eval`.
