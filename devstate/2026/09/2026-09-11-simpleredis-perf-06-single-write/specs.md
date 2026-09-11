# Specs
change: simpleredis-single-write-encode
- modified std_go_simpleredis_resp-commands
- modified std_go_simpleredis_tcp-session

## FindSpecHost (archive)

Search walked `openspec/specs/map.md` and every `openspec/specs/*/spec.md` plus `openspec/changes/**/specs/*/spec.md` on disk.

Search candidates:
- `std_go_simpleredis_resp-commands` (live owner)
- `std_go_simpleredis_tcp-session` (live owner)
- `std_go_reclaim_value-lifecycle`, `std_go_reclaim_context-lease`
- `std_go_tokenbucket_allow`, `std_go_tokenbucket_lua-eval`
- `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`
- families on `openspec/specs/map.md`: `std` / `go` / reclaim, simpleredis, tokenbucket, windowcounter
- in-flight deltas: `std_go_simpleredis_resp-commands`, `std_go_simpleredis_tcp-session`

verdicts:
  - { deltaId: std_go_simpleredis_resp-commands, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }
  - { deltaId: std_go_simpleredis_tcp-session, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands] }

## archive (2026-09-11)

FindSpecHost Search + Verdict. Same fold/high owners as propose. Live catalog folded ADDED encode framing, dual-engine proof, and encode benches into `std_go_simpleredis_resp-commands`; idle encode-scratch trim into `std_go_simpleredis_tcp-session`.
