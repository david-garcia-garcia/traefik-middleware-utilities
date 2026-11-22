# Specs
change: tokenbucket-expired-bucket-reclaim
- fold std_go_tokenbucket_allow
- skip std_go_tokenbucket_lua-eval — no live PTTL contract (integer EXPIRE already specified; e2e observes remaining TTL)
