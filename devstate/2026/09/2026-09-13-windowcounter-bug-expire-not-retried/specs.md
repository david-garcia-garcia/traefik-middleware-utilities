# Specs
change: windowcounter-exact-expire-if-no-ttl
FindSpecHost:
  - { deltaId: exact-expire-if-no-ttl, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }
  - { deltaId: std_go_windowcounter_sync-flush, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }
- modified std_go_windowcounter_sync-flush
