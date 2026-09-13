# Specs
change: reclaim-hook-panic-recovery
- modified std_go_reclaim_context-lease (fold, high — create panic/nil is the existing create-error path)
- modified std_go_reclaim_value-lifecycle (fold, high — panicking Sleep/Wake/Close end protocol)

FindSpecHost candidates: std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle. No new leaf.
