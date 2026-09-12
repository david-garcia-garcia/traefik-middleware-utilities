# Specs
change: windowcounter-buffered-flush-error
- modified std_go_windowcounter_sliding-take
- modified std_go_windowcounter_sync-flush

FindSpecHost:
- { deltaId: buffered-outage-on-take-peek, fold, spec-id: std_go_windowcounter_sliding-take, confidence: high, candidates: [std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush] }
- { deltaId: retain-flush-error-staleness, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }
