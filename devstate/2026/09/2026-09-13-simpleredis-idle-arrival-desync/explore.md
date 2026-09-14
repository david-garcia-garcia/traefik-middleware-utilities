# Explore
IssueKey: 2026-09-13-simpleredis-idle-arrival-desync

## Concepts

```
  GET k1  →  complete bulk  →  Buffered()==0  →  park
                    │
                    │  unsolicited bulk lands in kernel only
                    ▼
  GET k2  →  Buffered() still 0  →  write GET k2
                    │
                    ▼
              readReply decodes POISONED  (err==nil)
```

A **reply boundary** in dest `do` is `bufio.Reader.Buffered() == 0`. That is leftover already pulled into userspace. It is not bytes sitting only in the kernel receive queue. Dest already destroys the in-reader leftover (`2026-09-13-simpleredis-desync-boundary-check`). This ticket is the idle-arrival hole that change left as debt.

`borrow` / `takeIdleConn` pop a young idle socket with no read. `release` parks when `reusable` is true. A stray that arrives after the post-read check and before the next pre-write check is invisible to both `Buffered()` gates.

Identity (client address, user, tenant, Host, trust hop) is not set or reconstructed. The wrong-key Get is a decode/pool bug, not a reconstructed tenant fact.

## Decisions

- Reproduced on dest. `go test -tags simpleredis_bugs ./simpleredis/ -run TestBugIdleArrivalDesync` from the caller workspace: **FAIL** `Get(k2) returned "POISONED", want "v2"`; **19 wrong, 40 correct, 0 errors** over 59 later commands. Worse than the ticket's 6/59; same silent class (`err == nil`). Dest default-suite `startStrayExtraReplyFake` writes value+stray in one `Write`, so it does not cover this path.
- Baseline hot path (worktree, Windows amd64, `BenchmarkGet` `-benchtime 2s -count=3`): **18303–18796 ns/op**, 1024 B/op, 29 allocs/op.
- Option 1 as specified (`SetReadDeadline(time.Now())` then one-byte `Read`): throwaway loopback TCP on this Windows host. Empty socket: timeout in ~0s (cheap). Dirty socket (peer wrote `X` 50ms earlier, `Buffered()` still 0): **also timeout**, 0 bytes. A later blocking Read returned `X`. The expired deadline short-circuits before the kernel queue is inspected. **Not a correct probe on Windows.**
- Smallest deadline that *does* see kernel data here: `time.Now().Add(1ns)` and up. Dirty: immediate byte. Empty path: **~525 µs/op** (1ns and 1µs both; Windows timer resolution). Versus 18.6 µs Get that is **~28×**. `+1ms` empty path: **~1.52 ms/op** (~80×). Ticket forbids a hot-path regression. This is not a small extra syscall; it is a timer wait on the clean reuse path.
- `syscall.Conn` / `RawConn` / `MSG_PEEK` is how go-redis `connCheck` inspects the kernel queue on Unix (`internal/pool/conn_check.go` at pin `7f3b3dff`). Windows/non-unix `conn_check_dummy.go` is a **no-op** (`connCheck` returns nil). Traefik Yaegi registers `syscall` only when both `useUnsafe` flags are true (`knowledge/research/ext_traefik_plugins_useunsafe/`). This product must not set them. Option 1 cannot copy go-redis's peek.
- Option 2 (reply-shape vs command): the measured failure is GET bulk decoded as GET bulk (`POISONED` is a well-formed `$`). Shape correlation cannot distinguish those. It could catch GET vs INCR (bulk vs integer) and miss the rate-limiter window-key case. Not a fix for this defect.
- Option 3 (loud without preventing): same as option 2 unless a working probe exists. There is no loud path for a successful same-shape decode.
- Simplicity gate: there is no small, coherent, elegant, correct, Yaegi-safe, portable fix. **Stop after propose. Do not implement.** That stop is success. Keep `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md`. Do not add a default-suite test that would fail forever. Do not land the tagged `simpleredis_bugs` files.
- Residual TOCTOU (stray between a working probe and `writeCommand`) is real but moot if no probe ships. It is microseconds vs the idle-park window; not the reason to stop.
- Consume: `knowledge/devdocs/std_go_simpleredis.md` leftover gotcha already names in-reader destroy and the kernel-queue exclusion. No Language write this phase. Spec catch-up only if a fix is chosen; this run chooses none.
- Surgical vs siblings: a probe would live in `pool.go` `borrow` after `takeIdleConn`, not in `resp.go`. Moot if none is built.

## Open questions

- Q: Does option 1's expired-deadline one-byte read see kernel data on this OS, and is the extra syscall small enough vs `BenchmarkGet`?
  Rank: bounded asked — would change every idle reuse of existing `borrow`/`do` (call sites: `borrow` in `simpleredis/pool.go`, `do` in `simpleredis/resp.go`; searched `simpleredis/*.go` for `takeIdleConn`/`do(`/`runOnConn`; one borrow path, one `do`); criterion Desired 2–4 names option 1, Yaegi, and hot-path cost
  Decision: resolved — no. `SetReadDeadline(time.Now())` misses kernel data on Windows (dirty Read timed out; blocking Read then got the stray byte). The smallest deadline that sees data costs ~525 µs on the empty reuse path vs 18.6 µs Get. Not correct *and* cheap.
  By: explore

- Q: Can option 2 (reply-shape correlation) catch this defect for GET/GET?
  Rank: additive asked — new check this change would create; criterion Desired 2 names option 2
  Decision: resolved — no. Measured poison is a well-formed bulk. GET vs GET is the rate-limiter case. Shape cannot tell `v2` from `POISONED`.
  By: explore

- Q: Is there an elegant fix, or does the run stop after propose with none?
  Rank: additive asked — criterion Desired 2 and the simplicity gate name pick-at-most-one or none; stopping after propose is success
  Decision: assumed — none. Propose writes the three options and their costs and recommends no code change. Do not implement. Debt file stays.
  By: explore

- Q: Probe placement (`takeIdleConn`/`borrow` vs `do` before `writeCommand`) if a later human overrides the gate?
  Rank: additive incidental — means to option 1; requirement Unknowns names the two sites
  Decision: assumed — moot this run. If a human later commissions a probe, put it in `borrow` after `takeIdleConn` (parked sockets only, not fresh dials; does not touch `resp.go`).
  By: explore

- Q: Permanent test file name and fake prefix on dest?
  Rank: additive asked — new untagged test; criterion Desired 5
  Decision: assumed — moot this run (no fix, no failing default-suite test). If a later change ships a probe: `simpleredis/resp_test.go` plus `startIdleArrivalStrayFake` in `fake_redis_test.go` (must not collide with `startStrayExtraReplyFake` / untracked `startLateStrayFake`).
  By: explore
