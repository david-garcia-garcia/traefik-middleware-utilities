# Explore
IssueKey: 2026-09-12-simpleredis-bug-02-unbounded-reply-allocation

## Concepts

A RESP `$` or `*` header is a length the client trusts before any payload byte arrives. `readBulk` (`simpleredis/resp.go`) does `make([]byte, length+2)` then `io.ReadFull`. `readReply` `*` does `make([][]byte, count)` then reads each element. `parseLen` only rejects overflow past `maxParseLen` (`int` max). A length that fits in `int` and is still huge allocates or panics (`length+2` wrap on MaxInt). Truncated ReadFull after that allocate is EOF → `ioError` → `redis:unreachable`, which `shouldRetry` retries (`commands_exec.go`), so a lying header can repeat the allocate.

`errIssue` (`redis:issue?`) is not retried. Truncated *small* bulk stays unreachable (tcp-session spec). An over-cap header with no payload must fail as `errIssue` *before* `make`, so it does not take the EOF path.

Write-side already has `maxMSetEXPairs = 1024`. No read-side ceiling. Decode spec today requires `make([]byte, length+2)` plus `ReadFull` for accepted lengths; the cap sits in front of that `make`. Usage packet `knowledge/devdocs/std_go_simpleredis_resp-decode.md` has no ceiling. Research indexes have bulk-string wire form and go-redis `readLine`, not `proto-max-bulk-len` or a go-redis reader size limit (research delegated).

```
header $N / *N
    │
    ├─ parseLen fail / negative *  → errIssue, dirty
    ├─ bulk N < 0                 → errMiss
    ├─ N > package cap            → errIssue, dirty, no make   ← this change
    └─ else make + ReadFull
         └─ short read            → EOF → redis:unreachable, retry
```

## Decisions

- Package consts `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20`, not `Config` fields. After the bulk miss check, `length > maxBulkLength` returns `errIssue` without `make`. Array `count > maxArrayCount` returns `errIssue`, `clean == false`. Array `$` elements inherit via `readBulk` (one test: MGET-shaped `*` with an over-cap `$` element).
- Keep `make([]byte, length+2)` plus `io.ReadFull` for accepted lengths (decode spec).
- Proof on `readReply` with `bufio.Reader` / `strings.Reader` (same package, unexported). Cases: `$` and `*` just over each cap; MaxInt64 as header *digits*; lengths that currently panic; `$268435456\r\n` must not run `make`. Keep `TestParseLen` overflow-digit regression.
- Specs: `std_go_simpleredis_resp-decode` (ceiling in front of make) and `std_go_simpleredis_resp-commands` (over-cap header is `redis:issue?`, not truncated I/O).
- Do not take: bulk-trailer (bug-04), panic-leaks-pool-token, CI fuzz, permanent Fuzz, Config knobs, `readLine` remainder, cumulative array-reply budget (noted).

## Open questions

- Q: Do `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20` stay the numbers given Redis `proto-max-bulk-len` and `TestAllocDecodeBulk100KB`?
  Rank: additive asked — new package consts this change creates; requirement Desired 1 names both values
  Decision: resolved — keep those numbers. `TestAllocDecodeBulk100KB` (`simpleredis/bench_test.go`) is a 100 KB legitimate payload, well under 64 MiB. Ticket states Redis `proto-max-bulk-len` default 512 MB; this client is stricter by design, not a Config mirror. Research on the exact Redis/go-redis limits is delegated and must not raise the caps.
  By: explore

- Q: How to spell MaxInt64 / panic-length cases on 32-bit `int`?
  Rank: additive asked — requirement Desired 5 names MaxInt64 and panic lengths on `readReply`
  Decision: assumed — feed header digits from `strconv.FormatInt(math.MaxInt64, 10)`. On 32-bit, `parseLen` already returns false above `maxParseLen` (`errIssue`, `clean == false`) before the new cap. On 64-bit, `MaxInt` would panic in `make([]byte, length+2)` today (`length+2` wraps); the cap runs first. Do not skip the test. Do not pass `int(MaxInt64)` as a length on 32-bit.
  By: explore

- Q: Who already owns client identity (address, user, tenant, Host, trust hop) for this change?
  Rank: additive incidental — no identity reconstruct in this decode cap
  Decision: assumed — none. The cap classifies a RESP header; it does not set or rebuild a host fact.
  By: explore
