# Standards

1. [hard] Leave a trail — `simpleredis/commands_exec.go:72` — `runOnConn` adds a recover/log `MsgPanic`/re-panic defer after the release defer but the new block has no one-line intro naming that job
   → Intro the recover defer block (log then re-raise; release defer still runs first via LIFO)
   Status: done
   Argument: Intro on recover defer: log then re-raise.

2. [hard] Leave a trail — `simpleredis/resp.go:43` — pre-write leftover gate now branches on `handshakeCommand` to emit `MsgAuthLeftover`; the inner branch has no block intro separate from the outer leftover-refuse comment
   → Add a one-line intro on the handshake leftover Warn emit before `logWarn(MsgAuthLeftover, …)`
   Status: done
   Argument: Handshake leftover intro before MsgAuthLeftover.

3. [hard] Leave a trail — `simpleredis/slog_test.go:437` — new `levelGate` type has no job comment; reclaim documents the same helper at `reclaim/table_test.go:238`
   → Add the same job comment (drops lines below `min` so alloc tests prove Debug is gated)
   Status: done
   Argument: levelGate job comment matching reclaim.

4. [hard] Symmetry and consistency — `simpleredis/slog_test.go:19` — `recHandler` copies reclaim's capturing-handler shape but omits method job comments reclaim carries (`Enabled`, `WithAttrs`, `WithGroup`, plus helpers `records`/`dump`/`recLogger`/`requireMsg`/`attrString`)
   → Mirror reclaim's succinct method comments on the sibling helper methods
   Status: done
   Argument: recHandler method comments mirrored from reclaim.

5. [judgement] Duplicated Code — `simpleredis/pool.go:240` — dial AUTH and SELECT failure paths each close the socket and emit identical `MsgHandshakeFailed` when `err != errNoAuth`
   → Extract one handshake-dial-failure helper called from both branches
   Status: skipped
   Argument: judgement; AUTH and SELECT stay two explicit dial sites like dest.

6. [judgement] Duplicated Code — `simpleredis/resp.go:50` — `do` repeats the same `mapped == errTimeout` → `logDebugTimeout` guard after `writeCommand` and after `readReply` I/O mapping
   → Extract a small helper or single post-map hook so timeout logging lives once
   Status: skipped
   Argument: judgement; each I/O map is its own decision site.

7. [judgement] Leave a trail — `simpleredis/commands_msetex.go:86` — `storeGroupWrite` replaces `defer Unlock` with manual unlock before capability Debug emit with no comment on why the mutex scope changed
   → Note that logging runs outside `groupWriteMu` so Debug emit cannot hold the capability lock
   Status: skipped
   Argument: judgement; unlock-then-log is the #79 mutex-scope combine.
