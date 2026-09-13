# Specs
change: windowcounter-buffered-outage-local-cap

verdicts:
  - { deltaId: buffered-outage-nil-error, fold, spec-id: std_go_windowcounter_sliding-take, confidence: high, candidates: [std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush] }
  - { deltaId: flush-skip-not-return, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }

- modified std_go_windowcounter_sliding-take
- modified std_go_windowcounter_sync-flush
