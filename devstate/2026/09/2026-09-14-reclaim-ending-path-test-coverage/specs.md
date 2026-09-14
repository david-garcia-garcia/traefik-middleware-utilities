# Specs
change: reclaim-ending-path-test-coverage
verdicts:
  - { deltaId: reset-sleep-panic-and-unmap, fold, spec-id: std_go_reclaim_value-lifecycle, confidence: high, candidates: [std_go_reclaim_value-lifecycle, std_go_reclaim_context-lease] }
  - { deltaId: reset-sleep-panic-orphan-log, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }
- modified std_go_reclaim_value-lifecycle
- modified std_go_reclaim_context-lease
