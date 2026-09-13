## Why

A stray RESP bulk that lands in the kernel receive queue while a SimpleRedis socket is parked is invisible to both `Buffered()` gates in `do`. Later Gets can return another key's payload with `err == nil`. That is the worst class of failure for a rate limiter (admit/deny on another window key's count). Dest already destroys leftover that is already in the reader; this remaining hole has no small, portable, Yaegi-safe probe that is both correct and cheap.

## What Changes

- **No product code change.** The simplicity gate wins: stop after this proposal.
- Record the three evaluated directions and their measured costs (design.md). Recommend none.
- Leave `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md` open. Do not add a default-suite test that would fail forever. Do not land tagged `simpleredis_bugs` files. Do not edit `simpleredis/BUGS.md`.
- Live specs already exclude kernel-buffer idle arrival. Because behavior does not change, this change sets `skip_specs: true` and does not fold a new requirement.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. Spec-level behavior does not change. `skip_specs: true`.

## Impact

- No `simpleredis/` source or test edits.
- No spec delta for `std_go_simpleredis_tcp-session` or `std_go_simpleredis_resp-commands` (those leaves already exclude this hole).
- OpenSpec change `simpleredis-idle-arrival-desync` is the written recommendation for the human.
- Debt file on dest stays the follow-up if a later run commissions a probe despite the cost.
