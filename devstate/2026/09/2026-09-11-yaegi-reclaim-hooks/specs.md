# Specs
change: explicit-reclaim-lifecycle-hooks

FindSpecHost (Search + Verdict). MCP `opd-mcp` unavailable; Search walked `openspec/specs/map.md` and every `openspec/specs/*/spec.md` plus `openspec/changes/**/specs/*/spec.md` on disk.

Search candidates:
- `std_go_reclaim_value-lifecycle` (live owner; archive `2026-09-11-add-reclaim-table` delta)
- `std_go_reclaim_context-lease` (live owner; same archive delta)
- families on `openspec/specs/map.md`: `std` / `go` / `reclaim` only
- no misnamed ids, no in-flight change deltas besides this one (empty at Search)

verdicts:
  - { deltaId: std_go_reclaim_value-lifecycle, fold, spec-id: std_go_reclaim_value-lifecycle, confidence: high, candidates: [std_go_reclaim_value-lifecycle, std_go_reclaim_context-lease] }
  - { deltaId: std_go_reclaim_context-lease, fold, spec-id: std_go_reclaim_context-lease, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }

- fold std_go_reclaim_value-lifecycle (high; small adjustment: replace optional interfaces on the value with optional Hooks funcs; owner already names value-lifecycle)
- fold std_go_reclaim_context-lease (high; small adjustment: Open signature grows hooks; Close hook replaces Close() on the value; Yaegi load no longer MAY stay inert)
