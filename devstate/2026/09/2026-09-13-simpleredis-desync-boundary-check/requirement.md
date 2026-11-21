# Requirement
IssueKey: 2026-09-13-simpleredis-desync-boundary-check

## Problem
A pooled socket that still has unread RESP after one well-formed reply is returned to the idle pool. Later commands on that socket read the previous command's reply and return it as their own, with `err == nil`. The socket stays warm (`lastUsed` refreshed) so `IdleTimeout` never evicts it.

## Current (code)
- `simpleredis/resp.go` `do` — after `readReply`, returns `values, true, err` whenever the parsed value is well-formed. No `conn.reader.Buffered()` check.
- `simpleredis/resp.go` `readReply` — parses one RESP value; leftover bytes stay in `*bufio.Reader`. Over-cap `$`/`*`, bad CRLF, nested array, error-in-array, unknown type byte, mid-array timeout, and partial write already return `clean = false`.
- `simpleredis/commands_exec.go` `exec` — `sr.release(conn, reusable)` after `do`. A well-formed leftover therefore stays pooled.
- `simpleredis/pool.go` `release` — when `reusable` is true, sets `conn.lastUsed = time.Now()` then appends to `idleConns`.
- `simpleredis/pool.go` `takeIdleConn` — closes idle sockets only when `now.Sub(lastUsed) >= idleTimeout`. A socket that serves traffic never ages out.
- `simpleredis/commands.go` `Get` — returns the one slot `exec` decoded for `GET name`. No check that the bytes belong to `name`.
- `simpleredis/pool.go` `dial` — calls `do` for AUTH then SELECT and inspects only `err`. The `reusable` flag is discarded, so leftover after AUTH does not close the handshake socket.
- `simpleredis/pool_test.go` `TestConnectionIsReused` — 25 sequential Gets must open exactly 1 TCP connection.
- `simpleredis/commands_exec_test.go` `TestLoadingReplyIsRetried`, `TestTryAgainReplyIsRetried`, `TestRetryableRedisRepliesAreRetried` — `-LOADING ` / `-TRYAGAIN ` are complete error replies (`readReply` `clean = true`) and retry with the socket reusable.
- `simpleredis/pool_test.go` also holds `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestPeerClosedIdleConnEOFIsRetried`, `TestStaleIdleHeadIsClosedWhileTailStaysHot`, `TestTruncatedBulkIsUnreachableAndNotPooled`, `TestAuthAndSelectOncePerDial`.
- Dest `simpleredis/` has no `BUGS.md`, no `bugs_repro_test.go`, no `startStrayExtraReplyFake`, no `TestBugDesyncedSocketKeepsServingPreviousReplies`. Those exist only on `origin/bugfixes20260913`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — Get returns the bytes Set for that key, or `redis:miss`. No leftover-bytes / reply-boundary scenario.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — "Dirty reply is not returned to the idle pool" covers truncated bulk and other malformed heads, not an extra well-formed reply after a clean parse. Sequential Gets SHALL reuse one connection.
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` — fail-closed decode; no "Buffered() must be 0 before reuse".
- `knowledge/devdocs/std_go_simpleredis.md` and `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — no leftover-bytes destroy rule.

## Desired
1. In `do`, before `reusable = true`, require `conn.reader.Buffered() == 0`. Leftover bytes: return the decoded value to the caller (current command still succeeds) and `reusable = false` so `release` destroys the socket. Do not drain or resynchronise.
2. Permanent, untagged tests in the default `go test ./simpleredis/` suite: (a) port `TestBugDesyncedSocketKeepsServingPreviousReplies` so every Get returns its own key's value or an error, never another key's; (b) after a stray extra reply the idle pool is empty and the next command dials a new connection; (c) a compliant peer is not discarded (`TestConnectionIsReused` stays 25 Gets → 1 TCP connection).
3. Keep green: `TestLoadingReplyIsRetried`, `TestTryAgainReplyIsRetried`, `TestRetryableRedisRepliesAreRetried`, `TestReleaseKeepsSocketWhenLiveUnderCap`, `TestPeerClosedIdleConnEOFIsRetried`, `TestStaleIdleHeadIsClosedWhileTailStaysHot`, `TestTruncatedBulkIsUnreachableAndNotPooled`, `TestAuthAndSelectOncePerDial`, MGET / Eval / MSetEX array-decode tests.
4. Prefer the fix inside `resp.go` `do`. Touch `pool.go` / `commands_exec.go` only if a one-hunk need appears.
5. Package constraints from `openspec/specs/std_go_simpleredis_tcp-session/spec.md`: stdlib imports only; no `unsafe`, cgo, or generics; keep Yaegi workarounds (`context.AfterFunc`, no `errors.As` on a package-local struct, no `net.Error` assert). `Buffered()` is stdlib; add a Yaegi test if practical.
6. Do not create or edit `simpleredis/BUGS.md`.
7. Verify with `go vet ./simpleredis/` and `go test -count=1 -timeout 300s ./simpleredis/`. Do not require local `-race` (no C toolchain).

## Affected
- `simpleredis/resp.go` (`do`)
- `simpleredis/resp_test.go` and/or a new untagged test file (port of `startStrayExtraReplyFake` + regression + discard + no-churn)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` and/or `std_go_simpleredis_tcp-session` dirty-reply (propose adds leftover-bytes destroy)
- `knowledge/devdocs/std_go_simpleredis.md` / `std_go_simpleredis_resp-decode.md` if usage still implies a clean parse is always reusable
- Optional Yaegi cover in `simpleredis/yaegi_test.go`

## Out of scope
- Creating or editing `simpleredis/BUGS.md` (another agent).
- Reference-branch bugs 1 (Yaegi `handshakeFailure` matching) and 2 (lost in-use turn).
- Draining leftover bytes to resynchronise the socket.
- Sending `HELLO` or speaking RESP3.
- New dependencies (`go-redis`, miniredis, vendor pkgs), `unsafe`, cgo, generics.
- Local `go test -race` (no C toolchain; CI runs the race job).
- Other packages (`reclaim`, `tokenbucket`, `windowcounter`) and other agents' untracked paths.

## Unknowns
- Whether propose folds leftover-bytes destroy into `std_go_simpleredis_tcp-session` dirty-reply, into `std_go_simpleredis_resp-commands` Get, or both.
- Whether a dedicated Yaegi test for `Buffered()` lands this change or only compiled tests (ticket: add if practical).
- Whether AUTH leftover (dial ignores `reusable`) needs a handshake-specific assertion beyond `TestAuthAndSelectOncePerDial`.
- Permanent test file name on dest (reference is build-tagged `simpleredis/bugs_repro_test.go`; dest tests must be untagged).

## Tensions
- Ticket measured stray bulk `STRAYYY` and `Get(k5)` as first wrong answer. Reference `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` writes `$5\r\nSTRAY\r\n` after every 5th command (`k4`), so the first mismatch is `Get(k5)` reading `STRAY`. Port that repro, not the prose string.
- Ticket: a socket the decoder cannot prove is on a reply boundary MUST NOT keep serving. Dest dirty-reply spec covers truncated/malformed only. Ticket wins; spec/usage catch up in propose.
- Ticket: keep the fix inside `do`. `dial` ignores `reusable`, so leftover after AUTH does not close the handshake socket. Do not fail AUTH/SELECT solely because `reusable` is false (false positive). Destroy on a later command's `release` is enough unless a test proves otherwise.
- Compliant Redis 7 will not trigger this (client never sends `HELLO`). Ticket is honest. The ask is still the silent wrong-key consequence when a non-compliant peer or proxy injects an extra reply.
- Do not edit `simpleredis/BUGS.md` even though the reference lives there; another agent owns that file.
