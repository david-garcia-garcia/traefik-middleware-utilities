## Context

Dest `do` calls `SetDeadline` once for write plus the whole reply. `bufio.Reader`/`Writer` are created on `pooledConn.netConn` at dial, so a stall refresh must wrap that conn before `bufio`. Overall budget is already `bindCommandDeadline` plus `watchConnClose`. Proceed policies: `devstate/explore.md`. Caller vs library timeout owner is `clampTimeout` / `ioOrContext` / `libraryTimeout`.

## Goals / Non-Goals

**Goals:**
- `IOTimeout` bounds quiet time between kernel reads/writes, not total transfer size.
- A steadily streaming multi-megabyte bulk that outlasts one `IOTimeout` returns intact.
- A peer that goes silent mid-reply still times out promptly.
- A drip peer cannot hold an in-use turn past the overall command budget.
- Existing caller-deadline vs `redis:timeout` tests stay green.

**Non-Goals:**
- Changing `readBulk` allocation (sibling BUG-4).
- Shrinking `maxBulkLength` or adding a bandwidth model (deviation).
- A second timeout knob.
- Other packages, `syscall`, `net.Error` asserts.

## Decisions

1. **`stallConn` wrapping the TCP conn at dial, before `bufio`, with every `net.Conn` method declared.** Yaegi v0.16.1 does not treat promoted methods from an embedded `net.Conn` as satisfying that interface. `pooledConn.stall` holds the wrapper so `do` binds without a type assert. Alternative: wrap only the reader after `bufio` — `ReadFull` would not hit it. Alternative: `SetDeadline` in `readBulk` between chunks — that edits the allocation path and misses `readLine`.

2. **`do` assigns `ctx` and `stall = IOTimeout` onto the wrapper; it does not one-shot `SetDeadline`.** Start-of-command `ioBound := clampTimeout(ctx, IOTimeout)` stays for `ioOrContext`. Fallback one-shot `SetDeadline` if `netConn` is not a `stallConn` (tests that inject a raw conn).

3. **Leave `maxBulkLength` at `64 << 20`.** After stall-refresh, `IOTimeout` is not a size bound. Overall budget plus `watchConnClose` is the wall-time cap.

4. **Tests in `simpleredis/bug3_stall_deadline_test.go` with `bug3` helper prefix.** Avoids colliding with sibling branches' fakes.

## Risks / Trade-offs

- [A drip peer could pin a turn if only the read deadline refreshes] → Mitigation: `watchConnClose` still closes on ctx done; drip test asserts elapsed ≤ overall budget and `OverFrees()==0`.
- [Per-Read full `IOTimeout` overshoots a 40 ms caller deadline] → Mitigation: `clampTimeout(ctx, IOTimeout)` on each Read/Write.
- [Yaegi `net.Error` assert] → Mitigation: keep `errors.Is`; concrete `*stallConn` assert only.
- [BUG-4 conflict in `readBulk`] → Mitigation: do not touch allocation.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Rows live on `devstate/explore.md`.
