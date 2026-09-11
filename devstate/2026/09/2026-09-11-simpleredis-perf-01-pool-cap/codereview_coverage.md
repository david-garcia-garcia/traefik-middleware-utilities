# Test coverage

1. [hard] Assertion does not prove the job — `simpleredis/simpleredis.go:308-323` — `release` keeps a reusable socket when idle is full and live < poolSize; `TestReleaseKeepsSocketWhenLiveUnderCap` (`simpleredis/simpleredis_test.go:898-909`) asserts total accepts ≥ 9 then one reuse Get. Closing extras at `len(idle) >= maxIdleConns` still leaves 12 accepts and that reuse green
   → After overlapping Gets with poolSize 16, assert `len(idle) > 8`, or that nine sequential Gets after the burst do not dial
   Status: done
   Argument: TestReleaseKeepsSocketWhenLiveUnderCap now asserts len(idle) > 8.
2. [judgement] Happy path only — `simpleredis/simpleredis.go:269-272` and `291-294` — `giveSlot` on closed-after-grant and dial fail; `TestCloseDrainsIdleAndDoesNotRepool` hits closed before the wait, `TestUnreachableHost` one failed dial
   → Assert eight failed dials then a live host still Gets, or Close during borrow does not starve later commands
   Status: skipped
   Argument: judgement; existing Close-before-wait and unreachable-host tests cover the fail-closed paths.
