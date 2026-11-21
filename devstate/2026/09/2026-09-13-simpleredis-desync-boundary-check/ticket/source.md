# A desynced pooled socket is never evicted and serves the PREVIOUS command's reply forever

Root cause: `do` in `simpleredis/resp.go` returns `reusable = true` whenever `readReply` parsed one well-formed value, and never checks whether the socket is actually drained. One extra unread reply shifts that socket one reply ahead PERMANENTLY. Every later command on it reads the previous command's reply and returns it as its own, with `err == nil`. There is no reply-boundary check, no sequence tag, and no self-heal: `release` in `simpleredis/pool.go` refreshes `conn.lastUsed` on every use, so a socket kept warm by traffic never reaches `IdleTimeout` and is never swept by `takeIdleConn`.

Measured: peer uses fully compliant RESP framing and answers `GET kN` with `vN`, but every 5th command appends one extra bulk reply. PoolSize 1, MaxRetries -1, keys k0..k39:
  first wrong answer Get(k5) = "STRAYYY", then Get(k6)->v5, Get(k7)->v6, Get(k8)->v7, ...
  35 of 40 answers wrong, ZERO errors raised, never healed.

Triggers: a RESP3 server push or invalidation message on a connection the client believes is RESP2; a duplicating or error-injecting proxy (Sentinel front end, twemproxy, Envoy, or this repo's own e2e RESP drop-relay); a server-side bug on a non-Redis engine. Be honest: the client never sends `HELLO`, so a compliant Redis 7 stays RESP2 and will not trigger this on its own, and a compliant-peer trigger was searched for and NOT found. The severity is the consequence: one tenant's cached value returned for another tenant's key, silently, until the process restarts.

Spec: openspec/specs/std_go_simpleredis_resp-commands (Get returns the value for its own key). A socket the decoder cannot prove is on a reply boundary MUST NOT keep serving commands.

Worth knowing: the decoder is otherwise strictly fail-closed already. Over-cap `$` and `*`, a bad CRLF trailer, a nested array, an error-in-array, an unknown type byte, a mid-array timeout, and a partial write ALL mark the socket dirty and destroy it. This is the one hole in that design.

Reference material (already pushed — read this first):
Branch `origin/bugfixes20260913` carries the bug report and a VERIFIED FAILING reproduction:
  git show origin/bugfixes20260913:simpleredis/BUGS.md
  git show origin/bugfixes20260913:simpleredis/bugs_repro_test.go
BUGS.md **section 3** is this bug. bugs_repro_test.go is build-tagged `//go:build bugrepro` and holds `TestBugDesyncedSocketKeepsServingPreviousReplies` plus its fake `startStrayExtraReplyFake`, confirmed to fail on current code for the right reason. Reuse that code as the starting point. Neither file exists on `master`; we are porting from that branch, not deleting anything. Do NOT create or edit simpleredis/BUGS.md — another agent owns it.

Agreed fix direction: In `do`, before returning `reusable = true`, require `conn.reader.Buffered() == 0`. Leftover bytes mean the peer is ahead of the protocol: return the decoded value to the caller (the reply itself was well-formed, so the current command still succeeds) but release the socket with `reusable = false` so it is DESTROYED instead of poisoning the pool.
Do NOT try to drain and resynchronise — there is no way to know how many replies to discard. Destroying the socket is the only sound recovery.

Invariants that MUST be preserved:
- A compliant peer writes one reply per command and leaves `Buffered() == 0` at that point, so this must NOT become connection churn. `TestConnectionIsReused` (25 sequential Gets -> exactly 1 TCP connection) is the guard. Verify it explicitly.
- A `-LOADING ...` reply leaves the socket clean and reusable and IS retried on the same socket — keep `TestLoadingReplyIsRetried`, `TestTryAgainReplyIsRetried`, and `TestRetryableRedisRepliesAreRetried` green.
- Keep green: `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestPeerClosedIdleConnEOFIsRetried`, `TestStaleIdleHeadIsClosedWhileTailStaysHot`, `TestTruncatedBulkIsUnreachableAndNotPooled`, `TestAuthAndSelectOncePerDial` (AUTH and SELECT run through the same `do`), and the MGET / Eval / MSetEX array-decode tests.
- Careful with the handshake: `dial` calls `do` for AUTH and again for SELECT on the same socket. A pipelined-looking handshake reply must not trip a false positive.

Tests that must land (permanent and untagged, running in the default suite):
1. Port `TestBugDesyncedSocketKeepsServingPreviousReplies` into a permanent regression test that PASSES after the fix: every Get returns its own key's value or an error, never another key's.
2. A test that the socket carrying a stray extra reply is DISCARDED — assert the idle pool is empty after that command and that the next command dials a new connection.
3. A test that a compliant peer does NOT get its socket discarded (no churn): sequential Gets on one connection stay on one connection.

Package constraints (from openspec/specs/std_go_simpleredis_tcp-session/spec.md):
- simpleredis source imports ONLY the Go standard library. No go-redis, no miniredis, no vendor pkgs.
- No `unsafe`, no cgo, no type parameters (generics).
- The package runs interpreted under Yaegi. Avoid constructs Yaegi breaks on: `errors.As` on a package-local struct, `net.Error` type asserts, and interpreted code selecting on a context channel from a goroutine (hence `context.AfterFunc` in resp.go). Keep those workarounds intact. `bufio.Reader.Buffered()` is plain stdlib and safe interpreted, but add a Yaegi test if you can.
- Session source keeps copy conversions: `[]byte(...)` and `string(...)`, no unsafe aliasing.

This agent owns: `simpleredis/resp.go`, `simpleredis/resp_test.go`, plus any new test file, plus `openspec/` for the spec update.
Prefer keeping the fix INSIDE `resp.go`'s `do`. If you must touch `pool.go` or `commands_exec.go`, keep the hunk minimal.
Do NOT create or edit `simpleredis/BUGS.md`.

Verification gates: `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/`. Race detector needs CGO and there is NO C toolchain here (`-race` fails locally). CI runs the race job.
