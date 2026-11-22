# Test coverage

1. [judgement] Happy path only — `backendbackoff/allow.go:180` — `moveToBackLocked` middle node (`entry.prev` set, not tail) untested
   Quote:
       ```
       production backendbackoff/allow.go:176-180: if entry == g.tail { return }; if entry.prev != nil { entry.prev.next = entry.next } else { g.head = entry.next }
       test backendbackoff/zzz_expiry_list_test.go:154-170 TestAllowDenyAndReportMoveToTailExpiryList: Allow("a") then Report("b") with list a,b,c,d — both calls move the then-head; no Allow/Report of a non-head, non-tail key
       ```
   Fix: Assert Allow or Report of a non-head, non-tail key moves it to the tail
   Status: skipped
   Argument: judgement; not applied unattended.
