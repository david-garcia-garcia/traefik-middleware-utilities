# Explore
IssueKey: 2026-09-12-simpleredis-risk-04-non-basic-resp2-replies-rejected

## Concepts

`readReply` (`simpleredis/resp.go`) is the only RESP decoder. Supported heads are `+`, `:`, `-`, `$`, and a flat `*` whose elements are `$`, `:`, or `+`. Everything else is `errIssue` with `clean == false`. `do` identity-compares `err == errIssue` before `ioError`; any new dirty protocol sentinel MUST pass through that gate or it becomes `redis:unreachable`.

`Eval` returns `[][]byte`. Lua indexed tables become RESP2 arrays; nested tables become nested arrays; `{ err = "..." }` becomes a `-` reply (`knowledge/research/ext_redis_eval/notes.md`). Production `tokenbucket/lua.go` wraps every slot in `tostring`, so the live script stays inside the flat bulk-string set.

`*-1` is a legal RESP2 null array (BLPOP timeout, EXEC abort). This client has no those verbs. Lua `false` is null bulk, not a null array (`knowledge/research/ext_redis_resp_null-array/notes.md`). Dest spec, usage gotcha, and `TestMalformedReplyIsIssueAndNotPooled` already pin `*-1` as `redis:issue?`, not miss, idle empty.

No `HELLO` on the send path. RESP3 type bytes can still arrive as garbage; they are unknown type bytes today.

Measured: `go test -short -count=1 -run 'TestMalformedReplyIsIssueAndNotPooled|TestEvalMixedArrayReply' ./simpleredis/` passed. Nested `*1\r\n*0\r\n`, `*-1`, unknown `?`, HTTP-shaped: `redis:issue?` and idle 0. Mixed `*3` of `$` / `:` / `+` succeeds. No test calls `readReply` for `clean` directly. `startStaticRedis` does not expose accept count; redial proof needs a counting listener (same pattern as `commands_exec_test.go` / `peerCloseFake.connections`).

```
Lua return
  tostring slots     -->  *n of $     -->  [][]byte   (tokenbucket today)
  numbers            -->  *n of :     -->  [][]byte   (already parsed)
  nested table       -->  *n of *     -->  errIssue, socket closed
  {err=...} in table  -->  *n of -     -->  errIssue, socket closed
  unknown / RESP3    -->  other byte  -->  errIssue, socket closed
```

## Decisions

Take the ticket's **minimum** scope only: document the Eval contract (flat arrays of bulk strings or integers; Lua authors wrap with `tostring`) and export a distinct `redis:unsupported-reply` for well-framed replies this decoder does not decode. Keep `clean == false` so the socket is discarded. Do not map `*-1` to miss. Do not add a nested `Reply` tree.

`do` must treat the new sentinel like `errIssue` (pass through, not `ioError`). `shouldRetry` already ignores it (not unreachable, not a retryable `-` prefix).

Malformed framing stays `redis:issue?`: empty line, missing CR, unparseable length, empty element line. Unsupported type bytes (including HTTP-shaped and RESP3), nested `*`, and `-` inside an array become `redis:unsupported-reply`. `*-1` stays `redis:issue?` (known type, count this client refuses; dest "Null array is not a miss").

Error() text is exactly `redis:unsupported-reply`. No type-byte suffix (callers match exact strings). Tests that pin `clean` call `readReply` with a `bufio.Reader`. Eval nested-array fake asserts accept count rose on the next command. Guard: three bulk strings matching tokenbucket's `tostring` triple.

Usage packet `knowledge/devdocs/std_go_simpleredis.md` is incomplete on Eval (no tostring / flat-array rule). Update it with the code and spec. No new research folder: EVAL mapping and `*-1` are already sourced.

## Open questions

- Q: Which of the three scopes does this run take?
  Rank: additive asked — new Eval comment, sentinel, and tests; requirement Desired line 1 names the minimum and forbids doing all three
  Decision: resolved — take the minimum (document Eval + `redis:unsupported-reply`, dirty socket). Skip optional `*-1` miss and nested-array trees.
  By: explore

- Q: Is a new exported sentinel allowed, or must Error() stay one of the five dest tokens?
  Rank: bounded asked — Desired #1 names `redis:unsupported-reply` or wrapping `errIssue`; dest spec "Exported error strings are stable" lists five tokens. Existing call sites enumerated (5 product/spec/doc + tests): `simpleredis/simpleredis.go`, `simpleredis/resp.go` (`readReply` defaults and `do` identity compare), `simpleredis/resp_test.go` `TestMalformedReplyIsIssueAndNotPooled`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (five-string requirement plus unknown-type / nested / bad-element scenarios), `knowledge/devdocs/std_go_simpleredis.md` five-string list. `windowcounter/limiter.go` constructs `RedisIssue` for integer parse, it does not match protocol type bytes — no change. Roots searched: `*.go` for `RedisIssue` / `redis:issue?`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`.
  Decision: resolved — export `RedisUnsupportedReply = "redis:unsupported-reply"` and extend the spec list to six. Wrap-with-suffix would break exact `Error()` match. `do` MUST pass the new sentinel through (today only `errIssue` avoids `ioError`).
  By: explore

- Q: Should `*-1` become `redis:miss` with `clean true`?
  Rank: bounded incidental — Desired #2 is optional; dest requirement "Null array is not a miss", usage gotcha, and `readReply` comment already pin `redis:issue?` idle empty because this client has no BLPOP/MULTI/EXEC. Call sites: those three plus `TestMalformedReplyIsIssueAndNotPooled` `null-array` row.
  Decision: resolved — keep dest. No current verb receives a null array. Lua `false` is `$-1`. Reversing would be a reshape dest already declined.
  By: explore

- Q: Does any planned limiter script need a nested table (gates a `Reply` type)?
  Rank: additive incidental — requirement Out of scope unless explore picks nested arrays; larger scope would change `[][]byte` and every caller. No limiter script in this tree returns nested tables. `tokenbucket/lua.go` is three `tostring` bulks. Planned `handoff-kong-window-limiter` / `handoff-leaky-bucket` are not in this worktree and are out of scope except as motivation to document the convention.
  Decision: resolved — do not recurse nested arrays. Document and fail `redis:unsupported-reply` instead.
  By: explore

- Q: How to surface the type byte if wrapping `errIssue` without a new public string?
  Rank: additive asked — Desired #1 offers wrapping as the alternative to a new token
  Decision: resolved — do not wrap. Exact sixth token. Operators match `redis:unsupported-reply`. Type byte stays off `Error()`.
  By: explore
