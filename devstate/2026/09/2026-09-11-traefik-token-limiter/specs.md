# Specs
change: add-tokenbucket
- added std_go_tokenbucket_allow
- added std_go_tokenbucket_lua-eval

FindSpecHost:
- { deltaId: allow, new, spec-id: std_go_tokenbucket_allow, confidence: high, candidates: [std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush] }
- { deltaId: lua-eval, new, spec-id: std_go_tokenbucket_lua-eval, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_windowcounter_sync-flush] }
