# Explore
IssueKey: 2026-09-13-simpleredis-bug-eval-null-bulk

## Concepts

RESP2 has two “nothing” shapes that this client already treats differently: null bulk `$-1` and null array `*-1`. Dest decode maps **every** top-level `$-1` to `errMiss` (`readBulk` length `< 0` → `errMiss`; `readReply` `$` forwards it). Array `$` on the same `errMiss` `continue`s and leaves `values[i]==nil`. Get never sees a nil slot because `exec` already returned the miss. Eval is a passthrough of that same decode, so Lua `return false` (official Lua→RESP2: false → null bulk) looks like a GET miss.

```
DestBranch
  $-1 ──readBulk──► errMiss
         │
         ├─ top-level $ ──► nil, true, errMiss  ── Get / Eval both see miss
         └─ array $      ──► values[i]==nil, nil error ── MGET already correct

Agreed
  $-1 ──readBulk──► errMiss (internal only)
         │
         ├─ top-level $ ──► [][]byte{nil}, true, nil
         │                     ├─ Get: values[0]==nil → errMiss
         │                     └─ Eval: one nil slot, error nil (not IsMiss)
         └─ array $      ──► unchanged aligned nil slots
```

Empty bulk `$0` is a non-nil empty slice (`data[:0]`). That stays a hit for Get, not a miss.

In-tree Eval callers (tokenbucket `allowScript`, windowcounter flush, MSetEX Lua fallback, probe `/eval`) wrap slots with `tostring` or return numbers. They are not the false-as-miss victims; the public Eval contract still must not report `redis:miss` for a successful Lua false/nil.

Usage today: `std_go_simpleredis.md` gotcha still says “Null bulk `$-1` is `redis:miss`.” After this change that sentence is only true for Get (verb mapping), not decode. `std_go_simpleredis_resp-decode.md` still says `parseLen` “Accept optional leading minus (`$-1` miss).” Spec `std_go_simpleredis_resp-decode` scenario “Null bulk remains a miss” is Get-shaped but lives on the decode leaf.

Research: `knowledge/research/ext_redis_eval/` lists `false` → null bulk; `return nil` is not a table row. Ticket still names both; the agreed how is the wire `$-1`, not a second Lua mapping. `ext_redis_resp_bulk-string` already separates GET miss `$-1` from empty `$0`.

Identity: this change does not set or reconstruct client address, user, tenant, Host, or trust hop. No owner question.

## Decisions

- Decode owns “null bulk is a nil slot.” Get owns “one nil slot is `errMiss`.” Eval does not remap `ErrMiss`. That is the ticket’s how; do not paper over at the verb.
- Tests first, then the two-line decode + Get mapping. Repro MUST fail on current dest under `go test -short ./simpleredis/` with no `//go:build bugrepro`.
- Fold into existing specs `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands`. No new family. Change name kebab from Desired: `simpleredis-eval-null-bulk-not-miss`.
- Adapt the caller’s Eval sample to dest `Eval(ctx, script, digest, keys, args)` via `ScriptSHA1Hex`. Do not change the public signature.
- Usage gotcha and decode packet catch up in implement / devdocsimpact so they match decode-is-slot / Get-maps-miss.

## Open questions

- Q: Does Lua `return nil` need a second test besides `return false` / wire `$-1`?
  Rank: additive asked — Desired names Eval return false / nil as RESP2 null bulk; criterion is the wire form
  Decision: assumed — one compiled `$-1` Eval test is the proof; both Lua forms share that wire on RESP2. Official table lists `false`; `nil` is the same slot.
  By: explore

- Q: After decode yields a nil slot, which existing verbs besides Get and Eval consume a top-level `$`?
  Rank: bounded asked — Desired maps Get miss and Eval slot; 2 top-level bulk verbs enumerated (`Get`, `Eval` in `simpleredis/commands.go` and `commands_eval.go`); MGET is the array path already nil-slot; SET/DEL/INCR/EXPIRE/MSetEX do not take top-level null bulk
  Decision: assumed — only Get maps `values[0]==nil` to `errMiss`; Eval returns the slot; MGET unchanged. No other verb rewrite.
  By: explore

- Q: Where do the reproducing tests live, and is a verb-level Get `$0` missing?
  Rank: additive asked — Desired implement order names `commands_eval_test.go` or `resp_test.go`; Unknowns name `$0` Get
  Decision: assumed — Eval `$-1` in `commands_eval_test.go`; Get `$-1` still miss stays on `TestGetHitAndMiss` (keep passing) plus a static-peer Get `$-1` if needed after decode; add Get `$0` not-miss via `startStaticRedis` if no verb-level test exists (`readBulk` empty-ok is not enough).
  By: explore

- Q: Should the decode spec keep “`$-1` remains `redis:miss`” as a decode rule?
  Rank: bounded asked — Desired: update specs if decode currently says $-1 remains miss for every verb; Get still maps miss; decode yields a nil slot. Fold into existing `std_go_simpleredis_resp-decode` (1 leaf) and commands Eval/Get scenarios
  Decision: assumed — rewrite decode: optional minus still parses; top-level `$-1` is a nil slot with nil error; Get scenario stays miss on commands spec. Do not leave “every verb is miss” on the decode leaf.
  By: explore
