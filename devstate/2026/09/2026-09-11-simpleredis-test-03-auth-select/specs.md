# Specs
change: simpleredis-test-03-auth-select
- modified std_go_simpleredis_tcp-session (fold, high) — handshake AUTH/SELECT failure closes, is not pooled, maps AUTH-class to redis:noauth, no redial; fake plus live skip-if-unset
- modified std_go_simpleredis_resp-commands (fold, high) — probe Password/Database; Pester wrong-password and SELECT 99 routes; keep no-password /redis /dragonfly SHALL

## archive FindSpecHost (2026-09-11)
Task subagent unavailable (cursor namespace has no Task). Search+verdict on archive worker.
verdicts:
  - { deltaId: std_go_simpleredis_tcp-session, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands] }
  - { deltaId: std_go_simpleredis_resp-commands, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }
