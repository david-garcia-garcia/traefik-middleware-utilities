# Specs
change: tokenbucket-ttl-whole-seconds

FindSpecHost:
- construction-whole-seconds: fold `std_go_tokenbucket_allow` (high) — candidates `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`
- agreement-by-reject: fold `std_go_tokenbucket_lua-eval` (high) — candidates `std_go_tokenbucket_lua-eval`, `std_go_tokenbucket_allow`

- modified std_go_tokenbucket_allow
- modified std_go_tokenbucket_lua-eval
