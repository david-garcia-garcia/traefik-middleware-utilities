# Specs
change: simpleredis-eval-caller-digest

FindSpecHost:
- delta `std_go_simpleredis_resp-commands`: fold `std_go_simpleredis_resp-commands` (high). Candidates: `std_go_simpleredis_resp-commands` (Eval requirement). Small adjustment to that leaf: caller digest + exported `ScriptSHA1Hex`. Not `std_go_simpleredis_live-e2e` (no Go signature). Not `std_go_tokenbucket_lua-eval` (explore: leave caller-surface EVALSHA sentence).

- modified std_go_simpleredis_resp-commands
