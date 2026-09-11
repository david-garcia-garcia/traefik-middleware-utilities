# Nitpicks

1. [hard] Name for the scope — `tokenbucket/limiter_test.go:79` — `TestMemory_RefundWhenWaitExceedsMaxDelay` names the post-refund Allow `again` and `wait2`; those are a placeholder and a numbered stem, not the role (allowed/wait after refund)
   → `allowedAfterRefund`, `waitAfterRefund`
   Status: done
   Argument: renamed to allowedAfterRefund / waitAfterRefund.
