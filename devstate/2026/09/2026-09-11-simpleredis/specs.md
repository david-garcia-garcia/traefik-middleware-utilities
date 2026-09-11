# Specs
change: add-simpleredis
- added std_go_simpleredis_tcp-session
- added std_go_simpleredis_resp-commands
- archived 2026-09-11-add-simpleredis

## FindSpecHost
OPD MCP was down; naming by `skill:opd-speclibrarian:FindSpecHost`. Search walked `openspec/specs/map.md`, `openspec/specs/*/spec.md`, and in-flight `openspec/changes/**/specs`. Candidates: `std_go_reclaim_context-lease`, `std_go_reclaim_value-lifecycle`. No `std_go_simpleredis_*` owner yet. Family `std` / domain `go` already in `openspec/specs/domains.md`.

verdicts:
  - { deltaId: std_go_simpleredis_tcp-session, new, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }
    Why: large new capability (new package `simpleredis/`). Reclaim leaves are a different component. 4th part names the TCP session job, not the change kebab.
  - { deltaId: std_go_simpleredis_resp-commands, new, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_reclaim_context-lease, std_go_reclaim_value-lifecycle] }
    Why: large new capability. RESP commands are a separate job from the session (One job, one owner). Not a fold into reclaim value-lifecycle. Proof/harness is not its own leaf.
