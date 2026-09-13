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

Later measurement narrows those two options, in favour of the deadline read:

- The `syscall` peek is **not** blocked by missing symbols, contrary to the assumption above. Yaegi v0.16.1 exports everything go-redis's `connCheck` uses (`Read`, `EAGAIN`, `EWOULDBLOCK`, `Recvfrom`, `MSG_PEEK`, `MSG_DONTWAIT`, `SetNonblock`, and `Conn` / `RawConn` with correctly shaped wrappers). It is blocked by deployment: Traefik registers `syscall` only when `useUnsafe` is true on both the manifest and the operator's static config, and manifest-true with operator-false makes Traefik refuse to load the plugin outright (`knowledge/research/ext_traefik_plugins_useunsafe/`). CI here also fails if `useUnsafe` flips true.
- A `syscall` peek is Unix-only and so wants a per-OS file split, which is a second trap: Yaegi ignores `//go:build`, so a `conn_check.go` / `conn_check_dummy.go` layout silently resolves to the no-op. Filename GOOS suffixes are the only mechanism that works (`knowledge/research/ext_traefik_plugins_yaegi-build-constraints/`).
- The zero-deadline `Read` needs none of that — no `unsafe`, no `syscall`, no build constraints, no manifest flag — and would catch both failures in one probe: `io.EOF` for peer close, `n > 0` for idle arrival, timeout for a healthy socket. Open question before it can be designed: whether an already-expired deadline makes the runtime return `ErrDeadlineExceeded` *before* it returns bytes that are already pending. If it does, the naive form detects peer close but silently misses the desync it is meant to catch, which is worse than not probing. That has to be proven, not assumed.
