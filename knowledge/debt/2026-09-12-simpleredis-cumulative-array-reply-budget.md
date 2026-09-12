# Cumulative array-reply decode budget

IssueKey: 2026-09-12-simpleredis-bug-02-unbounded-reply-allocation
Size: large
Action: note

## Why this follow-up
Per-element `maxBulkLength` plus `maxArrayCount` still allow `maxArrayCount` slots each at `maxBulkLength`. A lying `*` header under the count cap with large `$` elements could still allocate a huge total.

## Why it was not taken
The ticket listed a cumulative array-reply budget under “consider also”. This run’s ask is the two package consts and the header checks only.

## Risks
A crafted array under both caps can still exhaust memory. Legitimate SimpleRedis traffic (MGET fan-out, limiter scripts) stays far below either cap, so the residual is hostile or buggy peers, not normal verbs.
