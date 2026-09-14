# Specs
change: reclaim-finished-race-lock-defer

FindSpecHost:
- { deltaId: bind-finished-and-mutex-defer, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }
- archive reconfirm: { deltaId: std_go_reclaim_context-lease, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }

- modified std_go_reclaim_context-lease
