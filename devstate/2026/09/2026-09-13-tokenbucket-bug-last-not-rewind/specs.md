# Specs
change: tokenbucket-last-not-rewind

FindSpecHost:
- { deltaId: last-not-rewind-allow, fold, spec-id: std_go_tokenbucket_allow, confidence: high, candidates: [std_go_tokenbucket_allow, std_go_tokenbucket_lua-eval] }
- { deltaId: last-not-rewind-lua-hset, fold, spec-id: std_go_tokenbucket_lua-eval, confidence: high, candidates: [std_go_tokenbucket_lua-eval, std_go_tokenbucket_allow] }

- modified std_go_tokenbucket_allow
- modified std_go_tokenbucket_lua-eval
