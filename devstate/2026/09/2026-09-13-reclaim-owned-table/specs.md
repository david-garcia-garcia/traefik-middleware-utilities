# Specs
change: reclaim-owned-table

verdicts:
  - deltaId: drop-process-table
    fold: std_go_reclaim_context-lease
    confidence: high
    candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle]
    why: Existing leaf already owns process-table SHALL, grace construction, and Traefik Open load. One–three requirement edits, not a new family.

- folded std_go_reclaim_context-lease
