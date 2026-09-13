# Idle-arrival desync still poisons a pooled SimpleRedis socket

IssueKey: 2026-09-13-simpleredis-desync-boundary-check
Size: large
Action: note

## Why this follow-up
`do` only refuses reuse when `bufio.Reader.Buffered()` is already non-zero. That reports userspace leftover from a prior `Read`, not bytes still sitting in the kernel receive buffer. A stray reply that arrives while a clean socket is idle in the pool is invisible at borrow. The next `Get` can decode that stray as its own reply with `err == nil`, and the socket may be pooled one reply ahead if the real reply has not been pulled yet.

## Why it was not taken
Closing idle arrival needs a readability probe at borrow (a zero-deadline read, or a `syscall`-level peek). That is a new session design, awkward to keep Yaegi-safe and stdlib-only, and was out of scope for the leftover-in-reader gate this change ships.

## Risks
One tenant's cached bytes can still be returned for another tenant's key after a RESP3 push, invalidation, or proxy duplicate on an idle pooled socket. The default-suite fake writes value and stray in one `Write`, so it does not fail if this path stays open.

## Context
Borrow-time options for a later ticket: a zero-deadline `Read` on the idle socket before the next command is written, or a `syscall` peek of the kernel receive buffer. Both need a Yaegi-safe, stdlib-only design and their own proofs. Current gate: leftover already in `conn.reader` at the reply boundary (`simpleredis/resp.go` `do`).
