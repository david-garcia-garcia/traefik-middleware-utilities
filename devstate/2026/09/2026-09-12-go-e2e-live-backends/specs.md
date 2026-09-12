# Specs
change: add-go-e2e-live-backends

## FindSpecHost
Search walked `openspec/specs/map.md`, `openspec/specs/*/spec.md`, and in-flight `openspec/changes/add-go-e2e-live-backends/specs`. Candidates: `std_go_simpleredis_resp-commands`, `std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-decode`, `std_go_windowcounter_sync-flush`, `std_go_tokenbucket_lua-eval`. No `std_go_ci_*` owner. Family `std` / domain `go` already in `openspec/specs/domains.md`.

verdicts:
  - { deltaId: std_go_ci_test-suites, new, spec-id: std_go_ci_test-suites, confidence: high, candidates: [] }
    Why: large new capability (new CI job + new `knowledge/devdocs` leaf). 4th part names the suite catalog, not the change kebab.
  - { deltaId: std_go_simpleredis_live-e2e, new, spec-id: std_go_simpleredis_live-e2e, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }
    Why: past one–three requirements (engine-success verbs + Yaegi live). Existing leaves own MSetEX live and pool/peer-close, not the expanded verb suite.
  - { deltaId: std_go_simpleredis_resp-commands, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands] }
    Why: small adjustment to the existing MSetEX live SHALL (CI job + one-addr fail).
  - { deltaId: std_go_simpleredis_tcp-session, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session] }
    Why: small adjustment to live cap and CLIENT KILL SHALLs (e2e job + skip/fail).
  - { deltaId: std_go_windowcounter_sync-flush, fold, spec-id: std_go_windowcounter_sync-flush, confidence: high, candidates: [std_go_windowcounter_sync-flush] }
    Why: existing leaf owns live Redis/Dragonfly; extra Peek scenarios and skip/fail are a section on that requirement.
  - { deltaId: std_go_tokenbucket_lua-eval, fold, spec-id: std_go_tokenbucket_lua-eval, confidence: high, candidates: [std_go_tokenbucket_lua-eval] }
    Why: existing live SHALL; refund scenario and skip/fail are a section on that requirement.
