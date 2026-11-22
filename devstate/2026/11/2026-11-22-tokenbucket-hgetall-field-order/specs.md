# Specs

change: tokenbucket-hmget-named-fields

verdicts:
  - deltaId: std_go_tokenbucket_lua-eval
    verdict: fold
    spec-id: std_go_tokenbucket_lua-eval
    confidence: high
    candidates: [std_go_tokenbucket_lua-eval, std_go_tokenbucket_allow, std_go_tokenbucket_status]

- fold std_go_tokenbucket_lua-eval — named HMGET replaces positional HGETALL
