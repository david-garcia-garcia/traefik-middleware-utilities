# Specs
change: simpleredis-export-error-sentinels

FindSpecHost:
- { deltaId: export-sentinels-and-predicates, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands] }
- { deltaId: pool-wait-not-retried, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session] }
- { deltaId: get-miss-ismis, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }

- modified std_go_simpleredis_resp-commands
- modified std_go_simpleredis_tcp-session
- modified std_go_windowcounter_sync-flush
