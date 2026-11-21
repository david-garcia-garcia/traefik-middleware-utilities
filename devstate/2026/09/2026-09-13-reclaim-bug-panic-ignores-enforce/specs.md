# Specs
change: reclaim-panic-enforce-close

FindSpecHost:
- { deltaId: panic-close-order, fold, spec-id: std_go_reclaim_value-lifecycle, confidence: high, candidates: [std_go_reclaim_value-lifecycle, std_go_reclaim_context-lease] }
- { deltaId: panic-ending-mapped, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }

- modified std_go_reclaim_value-lifecycle
- modified std_go_reclaim_context-lease
