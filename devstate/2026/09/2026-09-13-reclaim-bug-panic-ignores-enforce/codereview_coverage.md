# Test coverage

1. [hard] Critical path untested — `reclaim/table.go:207` — enforced Wake-panic records `incarnation.createErr` then `close(oldReady)` so waiters already parked on that transition replay the wrapping error; `TestRepro_WakePanicUnmapsBeforeCloseWithEnforce` only starts a later `Open` after Close entered (`(none)` parked on Wake)
   → Assert a second `Open` already parked on the panicking Wake receives the wrapping Wake error and does not create while Close is blocked
   Status: done
   Argument: TestTable_WakePanicWaiterReceivesErrorWithEnforce parks a waiter on Wake; it gets the wrapping error and does not create.
