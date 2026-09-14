# Specs
change: simpleredis-allocation-free-encode
- modified std_go_simpleredis_resp-commands

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

FindSpecHost Search + Verdict. Same fold/high owners as propose. Live catalog folded ADDED encode framing, dual-engine proof, and the allocation-free framing requirement into `std_go_simpleredis_resp-commands`.

The `std_go_simpleredis_tcp-session` delta was dropped after the encoder was reduced to variant D: the idle encode-scratch trim it described belonged to the rejected single-write shape, so that leaf is unchanged from `master` and still holds 27 requirements.
