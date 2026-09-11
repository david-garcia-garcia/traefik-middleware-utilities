# Specs
change: retry-only-idempotent-commands

FindSpecHost:
- delta dead-pool-retry-by-verb → fold `std_go_simpleredis_tcp-session` (high). Candidates: `std_go_simpleredis_tcp-session` (retry owner), `std_go_simpleredis_resp-commands` (shapes only; do not duplicate).

- modified std_go_simpleredis_tcp-session
