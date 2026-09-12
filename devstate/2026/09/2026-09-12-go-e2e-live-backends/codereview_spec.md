# Spec

1. [wrong] `openspec/changes/add-go-e2e-live-backends/specs/std_go_simpleredis_live-e2e/spec.md` — ### Requirement: Live Redis and Dragonfly prove engine-success verbs — "Malformed RESP, truncated replies, LOADING retry, AUTH, and SELECT MUST remain fake-TCP only."
   Status: done
   Argument: live-e2e SHALL now names SELECT 99 and WRONGPASS; deviations.md taken. Spec aligned with DestBranch handshake proof.

2. [wrong] `knowledge/devdocs/std_go_test-suites.md:56` (Gotchas) — documents "Auth/SELECT handshake failure is Go E2E" with SELECT 99 and WRONGPASS on live addrs, contradicting the same rank-1 MUST (fake-TCP only for AUTH and SELECT).
   Status: done
   Argument: Gotcha already matched DestBranch; live-e2e SHALL now matches that packet.
