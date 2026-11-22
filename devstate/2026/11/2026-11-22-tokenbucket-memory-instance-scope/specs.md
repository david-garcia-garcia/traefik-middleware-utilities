# Specs
change: tokenbucket-memory-instance-scope

FindSpecHost (Live catalog: remaining live behavior — Memory per instance, Redis per key):

Search candidates:
- `std_go_tokenbucket_allow` — family `std_go_tokenbucket`, leaf `allow`. Allow contract, opaque `key`, caller-owned prefix. High overlap.
- `std_go_tokenbucket_lua-eval` — Eval script identity. No instance-scope promise.
- `std_go_tokenbucket_status` — `Status` fields. No instance-scope promise.
- In-flight `reclaim-alias` and archive tokenbucket deltas — not this job.

verdicts:
  - { deltaId: limiter-instance-scope, fold, spec-id: std_go_tokenbucket_allow, confidence: high, candidates: [std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval, std_go_tokenbucket_status] }

proposal.md: Modified `std_go_tokenbucket_allow` (fold). New: none. Skip: none.

- folded std_go_tokenbucket_allow
