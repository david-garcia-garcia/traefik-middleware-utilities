# Specs
change: tokenbucket-ttl-refill-floor

verdicts:
  - { deltaId: idle-ttl-floor, fold, spec-id: std_go_tokenbucket_allow, confidence: high, candidates: [std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval, std_go_tokenbucket_status] }
  - { deltaId: expire-seconds, fold, spec-id: std_go_tokenbucket_lua-eval, confidence: high, candidates: [std_go_tokenbucket_lua-eval, std_go_tokenbucket_allow, std_go_tokenbucket_status] }

- modified std_go_tokenbucket_allow
- modified std_go_tokenbucket_lua-eval
