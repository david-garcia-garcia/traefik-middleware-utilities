# Explore
IssueKey: 2026-09-13-simpleredis-per-read-deadline

## Concepts

`IOTimeout` today is a **total-transfer** bound: `do` calls `SetDeadline(now+clampTimeout(ctx, IOTimeout))` once, then write + whole reply must finish before that instant. A 4 MiB bulk that is still moving after that instant is `redis:timeout` forever.

The decoder's `maxBulkLength` (64 MiB) is a **parse cap**, not a time cap. Under total-transfer semantics those two numbers fight. Under **stall** semantics they do not: `IOTimeout` bounds quiet time between kernel reads; size is bounded by `maxBulkLength` and by the overall command budget `(MaxRetries+1)*(DialTimeout+IOTimeout)` which `watchConnClose` enforces by closing the socket.

`bufio.Reader` is created on `pooledConn.netConn` at dial (`pool.go`). A stall refresh must sit **under** that reader: wrapping later cannot see `Read`s `io.ReadFull` already issues through the bufio. The exclusive in-use turn makes mutating the wrapper's `ctx` / stall limit on each `do` safe.

`clampTimeout` + `ioOrContext` + `libraryTimeout` are the owner of caller-`DeadlineExceeded` vs `redis:timeout`. A per-Read deadline that is always `IOTimeout` (not remaining ctx) would overshoot a 40 ms caller deadline until `watchConnClose` fires. Re-clamping with `clampTimeout(ctx, IOTimeout)` on each `Read`/`Write` keeps that owner.

## Decisions

- Reproduce first: tagged `TestBugValueLargerThanIOTimeoutIsPermanentlyUnfetchable` on the caller checkout **failed** 5/5, `redis:timeout`, 5 dials, 20.5 MiB allocated, 0.30s. Matches PRODUCTION-BUGS.md BUG-3.
- Smallest correct shape: a tiny `net.Conn` wrapper installed at dial, before `bufio.NewReader`/`NewWriter`. `Read` and `Write` call `SetReadDeadline` / `SetWriteDeadline` with `clampTimeout(ctx, IOTimeout)` then the embedded conn. `do` stops the one-shot `SetDeadline`. AUTH/SELECT already go through `do`, so handshake gets the same bound.
- Overall command budget stays `bindCommandDeadline` + `watchConnClose`. A 1-byte-every-50ms drip cannot pin the turn past that ctx: `AfterFunc` closes the socket; `runOnConn` maps `contextStop` and `release`s with `reusable=false`.
- Do not change `readBulk` allocation (`make` + `ReadFull`). Sibling BUG-4 owns that.
- Do not add a second timeout knob. The Config field stays `IOTimeout`; its meaning becomes stall (quiet-peer), not total transfer. Comment and spec wording move; the name does not.
- Simplicity gate: this wrapper plus two tests plus spec/usage wording is small, coherent, and elegant. **Proceed to propose and implement.** Stopping is not required.
- Usage packets `std_go_simpleredis.md` (per-command `SetDeadline`) and `std_go_simpleredis_resp-decode.md` (keep `maxBulkLength` 64 MiB, do not put caps on Config) are enough to call the subsystem. Language has no gap that this run will invent. Produce in later phases: usage sentence for stall vs total-transfer only.

```
  dial → stallConn{Conn: tcp} → bufio.Reader/Writer
                │
  do: set ctx + stall limit (IOTimeout)
                │
  Read/Write → clampTimeout(ctx, IOTimeout) → Set*Deadline(now+bound) → kernel
                │
  ctx done → watchConnClose → Close → Read returns → contextStop / libraryTimeout
```

## Open questions

- Q: Should `maxBulkLength` shrink to what `IOTimeout` can carry?
  Rank: bounded incidental — 3 existing call sites enumerated (`resp.go` const, `readBulk` compare, `resp_test.go` over-cap cases); requirement Desired names reconcile, which is a means to stop advertising undeliverable sizes, not a new cap owner
  Decision: assumed — leave `64 << 20`. After stall-refresh, `IOTimeout` is not a size bound. A static shrink or a bandwidth model would re-break large values the stall fix makes readable, and would fight `std_go_simpleredis_resp-decode` (caps stay off Config). Overall command budget plus `watchConnClose` is the wall-time cap; over-cap headers stay `redis:issue?`.
  By: explore

- Q: Where does the stall wrapper live so `bufio` actually hits it?
  Rank: bounded asked — 1 production `SetDeadline` in `do`; dial constructs reader/writer on `netConn` (`pool.go` `dial`); Desired names a wrapper around the connection or reader
  Decision: resolved — wrap at dial as `pooledConn.netConn`; `do` assigns `ctx` and the stall limit onto that wrapper. Do not wrap a second reader around `bufio`.
  By: explore

- Q: How does per-Read refresh keep the caller deadline vs `redis:timeout` split?
  Rank: additive asked — new wiring on the wrapper this change creates; Desired names `clampTimeout` and existing `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout`
  Decision: assumed — each `Read`/`Write` uses `clampTimeout(ctx, IOTimeout)` so remaining caller/library time wins over a full stall window. `do` still computes start-of-command `ioBound` for `ioOrContext`. `watchConnClose` remains the hard stop. Do not type-assert `net.Error`.
  By: explore
