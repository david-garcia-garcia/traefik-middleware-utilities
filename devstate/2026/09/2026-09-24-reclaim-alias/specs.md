# Specs
change: reclaim-alias

FindSpecHost (Search + Verdict). Search walked `openspec/specs/map.md`, live `openspec/specs/std_go_reclaim_*/spec.md`, and archived reclaim deltas. Live catalog: alias weak refs, non-binding `Peek`, and incarnation-end alias clear are new public promises — not absence-only.

Search candidates:
- `std_go_reclaim_value-lifecycle` (live owner of reclaim table lifecycle and hooks)
- `std_go_reclaim_context-lease` (bind/Open ctx rules; no alias API today)

verdicts:
  - { deltaId: std_go_reclaim_value-lifecycle, fold, spec-id: std_go_reclaim_value-lifecycle, confidence: high, candidates: [std_go_reclaim_value-lifecycle, std_go_reclaim_context-lease] }

- fold std_go_reclaim_value-lifecycle (high; ADDED alias, Peek, incarnation-end alias clear)
