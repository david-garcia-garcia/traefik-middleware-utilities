# Specs

change: split-go-e2e-engine-jobs

FindSpecHost (before each write):

- fold `std_go_ci_test-suites` high — candidates: std_go_ci_test-suites — CI job count and skip/fail are this leaf
- fold `std_go_simpleredis_live-e2e` high — candidates: std_go_simpleredis_live-e2e — live skip and AUTH pair
- fold `std_go_simpleredis_resp-commands` high — candidates: std_go_simpleredis_resp-commands — live MSetEX CI env
- fold `std_go_simpleredis_tcp-session` high — candidates: std_go_simpleredis_tcp-session — pool-wait / CLIENT KILL CI job
- fold `std_go_windowcounter_sync-flush` high — candidates: std_go_windowcounter_sync-flush — live skip and CI job
- fold `std_go_tokenbucket_lua-eval` high — candidates: std_go_tokenbucket_lua-eval — live skip and CI job
