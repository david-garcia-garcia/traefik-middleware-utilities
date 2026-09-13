# Specs
change: tokenbucket-reject-nonfinite-rate
- fold std_go_tokenbucket_allow (high) — construction fail for non-finite rate is a small adjustment to the existing New-rejects-invalid-clock leaf. Candidates: `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval` (Lua fill out of scope).
