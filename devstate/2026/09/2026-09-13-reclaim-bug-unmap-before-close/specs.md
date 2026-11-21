# Specs
change: reclaim-close-before-unmap

FindSpecHost:
- { deltaId: std_go_reclaim_value-lifecycle, fold, spec-id: std_go_reclaim_value-lifecycle, confidence: high, candidates: [std_go_reclaim_value-lifecycle, std_go_reclaim_context-lease] }
- { deltaId: std_go_reclaim_context-lease, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }

- modified std_go_reclaim_value-lifecycle
- modified std_go_reclaim_context-lease
