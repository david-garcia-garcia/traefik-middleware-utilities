slidingAt only rejects windowSec<1. 1500ms is accepted and silently uses 1-second Redis buckets. Spec: window SHALL be a whole number of seconds.

Agreed how: Reject if window < time.Second OR window%time.Second != 0. Do not truncate. Weight and TTL use windowSec only. TestRepro_FractionalWindowAccepted: 1500ms Take must error. Keep 500ms rejection.

Implement order (required):
1. CREATE the failing repro FIRST. Example: `windowcounter/repro_fractional_window_test.go` (subtest A is the lock: 1500ms must error). Confirm FAIL on unfixed code (Take succeeds).
2. Then implement slidingAt validation + weight denom from windowSec.
3. Confirm that test PASSES and `go test -short -count=1 -timeout 60s ./windowcounter` passes. Keep TestTake_SubSecondWindow.

Bound the ask: only this bug.
