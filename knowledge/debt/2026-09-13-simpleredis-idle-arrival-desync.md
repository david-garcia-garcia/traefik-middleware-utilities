# Idle-arrival desync still poisons a pooled SimpleRedis socket

IssueKey: 2026-09-13-simpleredis-desync-boundary-check
Size: large
Action: note

## Why this follow-up
`do` only refuses reuse when `bufio.Reader.Buffered()` is already non-zero. That reports userspace leftover from a prior `Read`, not bytes still sitting in the kernel receive buffer. A stray reply that arrives while a clean socket is idle in the pool is invisible at borrow. The next `Get` can decode that stray as its own reply with `err == nil`, and the socket may be pooled one reply ahead if the real reply has not been pulled yet.

## Why it was not taken
Closing idle arrival needs a readability probe at borrow (a zero-deadline read, or a `syscall`-level peek). That is a new session design, awkward to keep Yaegi-safe and stdlib-only, and was out of scope for the leftover-in-reader gate this change ships.

## Risks
One tenant's cached bytes can still be returned for another tenant's key after a RESP3 push, invalidation, or proxy duplicate on an idle pooled socket. The default-suite fake writes value and stray in one `Write`, so it does not fail if this path stays open. That risk is now knowingly accepted rather than merely deferred: a well-behaved Redis does not emit the stray, and the leftover-in-reader destroy still covers extras that arrive in the same write.

## Context
Borrow-time options for a later ticket: a zero-deadline `Read` on the idle socket before the next command is written, or a `syscall` peek of the kernel receive buffer. Both need a Yaegi-safe, stdlib-only design and their own proofs. Current gate: leftover already in `conn.reader` at the reply boundary (`simpleredis/resp.go` `do`).

Later measurement narrows those two options, in favour of the deadline read:

- The `syscall` peek is **not** blocked by missing symbols, contrary to the assumption above. Yaegi v0.16.1 exports everything go-redis's `connCheck` uses (`Read`, `EAGAIN`, `EWOULDBLOCK`, `Recvfrom`, `MSG_PEEK`, `MSG_DONTWAIT`, `SetNonblock`, and `Conn` / `RawConn` with correctly shaped wrappers). It is blocked by deployment: Traefik registers `syscall` only when `useUnsafe` is true on both the manifest and the operator's static config, and manifest-true with operator-false makes Traefik refuse to load the plugin outright (`knowledge/research/ext_traefik_plugins_useunsafe/`). CI here also fails if `useUnsafe` flips true.
- A `syscall` peek is Unix-only and so wants a per-OS file split, which is a second trap: Yaegi ignores `//go:build`, so a `conn_check.go` / `conn_check_dummy.go` layout silently resolves to the no-op. Filename GOOS suffixes are the only mechanism that works (`knowledge/research/ext_traefik_plugins_yaegi-build-constraints/`).
- The zero-deadline `Read` needs none of that — no `unsafe`, no `syscall`, no build constraints, no manifest flag — and would catch both failures in one probe: `io.EOF` for peer close, `n > 0` for idle arrival, timeout for a healthy socket. Open question before it can be designed: whether an already-expired deadline makes the runtime return `ErrDeadlineExceeded` *before* it returns bytes that are already pending. If it does, the naive form detects peer close but silently misses the desync it is meant to catch, which is worse than not probing.

That open question has since been measured, and the answer closes the deadline read off as well. Throwaway harness on loopback TCP, this Windows host, 2026-09-13:

| Probe deadline | Dirty socket (stray byte in the kernel, `Buffered() == 0`) | Cost per reuse of a clean socket |
| --- | --- | --- |
| `SetReadDeadline(time.Now())` | timeout, 0 bytes — a later blocking `Read` then got the stray. **Misses the desync.** | ~0 |
| `time.Now().Add(1ns)` — smallest deadline that saw the byte | byte returned immediately | ~525 µs/op |
| `time.Now().Add(1ms)` | byte returned immediately | ~1.52 ms/op |

`BenchmarkGet` on the same host is 18303–18796 ns/op (~18.6 µs), so the cheapest form that actually sees the stray taxes every idle reuse about 28× a whole Get. The free form is the one the open question feared: it reports a clean socket while the stray is still queued. There is no setting of that deadline that is both correct and cheap.

Two directions that avoid a probe were evaluated at the same time and rejected on correctness, not on cost:

- Reply-shape correlation. The reproduced failure decodes a GET bulk as a GET bulk — the stray `POISONED` is a well-formed `$`. Shape cannot tell the real value from the stray. It would only catch a desync across reply *types* (bulk vs integer), which is not the rate-limiter window-key case, and only after the wrong data was already returned.
- "Make it loud" without a detector. A same-shape successful decode produces no error to promote. Being loud presupposes a working probe or a correlation that this failure evades, so it is not a third independent mechanism.

Recommendation as of 2026-09-13: do nothing. No small, coherent, Yaegi-safe, portable fix exists at an acceptable cost, and landing a default-suite test for a hole that stays open would only add a forever-red test. A human may still override by accepting the ~0.5 ms Windows reuse tax, or a Unix-only `syscall` peek behind `useUnsafe`.

If a later ticket does commission a probe: put it in `borrow` after `takeIdleConn`, not in `resp.go` (siblings keep touching `resp.go`), and restore the socket deadline before `do` stamps the remaining command budget. A residual TOCTOU window between probe and `writeCommand` stays — microseconds, not the idle-park window. The default-suite fake would need a stray-after-park writer beside the existing single-`Write` one before such a probe could be proven.
