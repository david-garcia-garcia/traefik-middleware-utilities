# Specs
change: ratelimit-status
- modified std_go_tokenbucket_allow
  Verdict: fold (high) — MODIFIED "Allow maps admit, wait, and deny" (returns `Status`; wait reported as `Delay`/`RetryAfter`; `Status{}` on error); ADDED "Eval tokens that is not a finite number is not a consume" (`errEvalTokens`, script unchanged). Candidates: std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval, std_go_tokenbucket_status.
- added std_go_tokenbucket_status
  Verdict: new (high) — five requirements on the meaning of `tokenbucket.Status` fields (limit/remaining/period/reset, delay vs. retry-after, saturation, memory/Redis agreement, unit+Yaegi+live proof); past the 1–3 requirement fold threshold of std_go_tokenbucket_allow. Candidates: std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval.
- modified std_go_windowcounter_sliding-take
  Verdict: fold (high) — MODIFIED "Take admits until the sliding estimate exceeds the limit", "Sliding estimate uses current and previous windows", "Peek observes without increment" (return `Status`; `Estimated` is not `Remaining`). Candidates: std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush, std_go_windowcounter_status.
- added std_go_windowcounter_status
  Verdict: new (high) — six requirements on the meaning of `windowcounter.Status` fields (limit/period/remaining, fixed-window reset, closed-form retry-after, limit < 1, buffered view, unit+Yaegi+live proof). Candidates: std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush.
- no delta std_go_tokenbucket_lua-eval, std_go_windowcounter_sync-flush — no sentence on the return type; "allowed", "estimate", and "wait class" stay true as `Allowed`, `Estimated`, `Delay + RetryAfter`.
- skip test migration, benchmarks, README, devdocs — no live contract.
