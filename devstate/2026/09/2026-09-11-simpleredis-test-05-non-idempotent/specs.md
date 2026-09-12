# Specs
change: retry-only-idempotent-commands

FindSpecHost:
- delta dead-pool-retry-by-verb → fold `std_go_simpleredis_tcp-session` (high). Candidates: `std_go_simpleredis_tcp-session` (retry owner), `std_go_simpleredis_resp-commands` (shapes only; do not duplicate).
- archive: delta `std_go_simpleredis_tcp-session` → fold `std_go_simpleredis_tcp-session` (high). Candidates: `std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-commands`.

- modified std_go_simpleredis_tcp-session
