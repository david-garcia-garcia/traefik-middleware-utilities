## Context

See proposal.md Why. Dest `do` (`simpleredis/resp.go`) refuses reuse when `bufio.Reader.Buffered() != 0`. That is leftover already in userspace. `borrow` / `takeIdleConn` (`simpleredis/pool.go`) pop a young idle socket with no read. Go `net.Conn.SetReadDeadline(time.Time{})` means no timeout, not a zero-timeout poll. Yaegi loads `stdlib.Symbols` only; `syscall` needs Traefik's dual `useUnsafe` gate, which this product must not set.

## Goals / Non-Goals

**Goals:**

- Pick at most one of the three ticket directions, or none, from measured cost and correctness.
- Leave a written recommendation a human can override.

**Non-Goals:**

- Shipping a probe, a correlation table, or a default-suite regression that would fail forever.
- Changing live leftover-in-reader behavior.
- Importing `syscall` / `RawConn`.

## Decisions

### 1. Do not implement a reuse-time kernel probe (option 1)

Ticket option 1: before reuse, `SetReadDeadline` to an expired/zero deadline, one-byte `Read`; timeout = clean, data/EOF = destroy.

Measured on this Windows host (loopback TCP, throwaway, 2026-09-13):

| Probe | Dirty socket (data in kernel, `Buffered()==0`) | Empty reuse cost |
| --- | --- | --- |
| `SetReadDeadline(time.Now())` then `Read` | Timeout, 0 bytes. A later blocking Read got the stray byte. **Incorrect.** | ~0 |
| `time.Now().Add(1ns)` (smallest that saw data) | Immediate byte | **~525 µs/op** |
| `time.Now().Add(1ms)` | Immediate byte | **~1.52 ms/op** |

`BenchmarkGet` on the same machine: **18303–18796 ns/op** (~18.6 µs). Empty-path 1ns probe is ~28× that Get.

go-redis idle health (`connCheck` at pin `7f3b3dff`) peeks with `syscall.Recvfrom(..., MSG_PEEK|MSG_DONTWAIT)` on Unix and is a **no-op** on Windows (`conn_check_dummy.go` returns nil). That peek is not Yaegi-safe here.

A later human who still wants a probe: put it in `borrow` after `takeIdleConn`, not in `resp.go` (siblings also touch `resp.go`). Restore the deadline before `do` sets `IOTimeout`. Residual TOCTOU between probe and `writeCommand` remains (microseconds, not the idle-park window).

### 2. Do not add reply-shape correlation (option 2)

The reproduced failure is GET bulk decoded as GET bulk (`POISONED` is a well-formed `$`). Shape cannot tell `v2` from `POISONED`. GET vs INCR (bulk vs integer) is not the rate-limiter window-key case. Catches desync only after wrong data of a *different* type; does not prevent it.

### 3. Do not "make it loud" without a detector (option 3)

Same-shape successful decode has no error to promote. Loud requires a working probe or a correlation that this failure evades. Option 3 is not a third mechanism.

### 4. Recommendation: none

No small, coherent, elegant, correct, Yaegi-safe, portable fix. Stopping after propose is success. Debt file stays.

## Risks / Trade-offs

- [Silent wrong-data stays in production behind a misbehaving peer] → Mitigation: none this change. A well-behaved Redis does not emit the stray. Dest leftover-in-reader destroy still covers same-write extras. Human may later accept a ~0.5 ms Windows reuse tax or a Linux-only `syscall` peek (would need `useUnsafe`).
- [Ticket asked for a default-suite test that fails before the fix] → Mitigation: do not land a forever-red test. If a later change ships a probe, `startIdleArrivalStrayFake` in `fake_redis_test.go` plus a proof in `resp_test.go`.
- [Specs continue to exclude the hole] → Intentional. `skip_specs: true`. Folding a SHALL that dest cannot honour would lie.

## Migration Plan

None. No deploy, no rollback.

## Open Questions

None. Remaining human override is whether to accept option 1's Windows timer cost or a `useUnsafe` peek; that is an override of this recommendation, not a deferrable unknown.
