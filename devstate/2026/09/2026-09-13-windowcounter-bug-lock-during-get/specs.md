# Specs
change: windowcounter-unlock-during-get

FindSpecHost:
- { deltaId: buffered-take-no-wait-on-other-key-get, fold, spec-id: std_go_windowcounter_sliding-take, confidence: high, candidates: [std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush] }
- { deltaId: flush-eval-unlock-and-subtract-delta, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take] }

- modified std_go_windowcounter_sliding-take
- modified std_go_windowcounter_sync-flush
