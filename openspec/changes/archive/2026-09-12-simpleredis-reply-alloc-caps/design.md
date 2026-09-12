## Context

Dest `readBulk` (`simpleredis/resp.go`) parses the `$` length then `make([]byte, length+2)` plus `io.ReadFull`. `readReply` `*` does `make([][]byte, count)` with no count ceiling. `parseLen` already rejects overflow past `maxParseLen`. `errIssue` is not retried; EOF after a short `ReadFull` becomes `redis:unreachable` and is. See proposal.md for why. Proceed policies: `devstate/explore.md`. Research: `knowledge/research/ext_redis_resp_bulk-string/`.

## Goals / Non-Goals

**Goals:**
- Package consts in front of `make` so over-cap headers are `errIssue` and dirty.
- Same-package `readReply` tests that fail if `make` still ran on `$268435456`.
- Keep `make([]byte, length+2)` plus `ReadFull` for accepted lengths.

**Non-Goals:**
- `Config` knobs for the caps.
- Cumulative array-reply byte budget (noted as debt).
- Bulk CRLF trailer verification (neighbor ticket).
- Changing `shouldRetry` or `ioError`.
- A permanent Fuzz target.

## Decisions

1. **Package consts, not Config.** Ticket Desired names `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20`. Legitimate traffic (100 KB decode guard, MGET fan-out, `maxMSetEXPairs = 1024`) sits far below. Alternative: Config fields — rejected; Out of scope.

2. **Cap after miss, before make.** `length < 0` stays `errMiss`. Then `length > maxBulkLength` → `errIssue`. Array: after `!ok || count < 0`, then `count > maxArrayCount` before `make([][]byte, count)`. Alternative: cap inside `parseLen` — rejected; overflow and over-cap are different (`parseLen` false vs a legal int that is too big).

3. **`errIssue`, not IO.** `do` already keeps `errIssue` without `ioError`. Over-cap with no payload would otherwise `ReadFull` EOF → retry. Alternative: return a typed size error — rejected; callers match `redis:issue?`.

4. **Tests call `readReply` on `bufio.NewReader(strings.NewReader(...))`.** Same package. Header-only over-cap needs no TCP fake. `$268435456\r\n` allocation proof: the call MUST return `errIssue` without growing heap by ~256 MiB (`testing.AllocsPerRun` / `runtime.MemStats` around that call). MaxInt64 cases use `strconv.FormatInt(math.MaxInt64, 10)` digits (explore assumed). Alternative: only Get through `startStaticRedis` — insufficient for `clean == false` and the no-`make` assertion.

5. **MGET inherit is one array `$` element over the bulk cap.** `*1\r\n$<over>\r\n` through `readReply` (and a Get/MGet command path if cheap). Do not add a limiter-script fixture. Alternative: only top-level `$` — rejected; Desired names the array element path.

6. **Keep overflow-digit `TestParseLen`.** Do not replace it with the new cap tests. `9999999999999999999999999999999999999999` stays.

## Risks / Trade-offs

- [Just-over-cap `$` would still allocate ~64 MiB if the check is `>=` vs `>` wrong, or if tests only use MaxInt] → Mitigation: just-over (`maxBulkLength+1`) plus `$268435456` no-make proof.
- [32-bit `parseLen` already fails MaxInt64] → Mitigation: still expect `errIssue`; do not skip (explore assumed).
- [Cumulative array still huge] → Mitigation: debt note; do not take here.

## Migration Plan

Library behavior: hostile or buggy peers that announced huge `$`/`*` now get `redis:issue?` instead of OOM/retry. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
