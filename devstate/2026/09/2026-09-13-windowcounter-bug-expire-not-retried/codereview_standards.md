# Standards

1. [hard] Leave a trail — `windowcounter/repro_expire_not_retried_test.go:9` — the new test’s job comment says it “proves takeExact never retries EXPIRE after the first-hit EXPIRE fails”; the body asserts the opposite (second Take must send EXPIRE and succeed)
   → Rewrite the comment to the job the body actually proves: after a failed first expire, a later Take still sets TTL on a leftover no-TTL key
   Status: done
   Argument: comment rewritten to leftover no-TTL later Take still sets TTL (`4da382f`).
