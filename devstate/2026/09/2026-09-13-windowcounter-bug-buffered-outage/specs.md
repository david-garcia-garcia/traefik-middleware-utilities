# Specs
change: windowcounter-buffered-outage-local-cap

verdicts:
  - { deltaId: std_go_windowcounter_sliding-take, fold, spec-id: std_go_windowcounter_sliding-take, confidence: high, candidates: [std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush] }
  - { deltaId: std_go_windowcounter_sync-flush, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }

- modified std_go_windowcounter_sliding-take
- modified std_go_windowcounter_sync-flush
