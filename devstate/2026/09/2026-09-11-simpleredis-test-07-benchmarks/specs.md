# Specs
change: simpleredis-hot-path-benchmarks

FindSpecHost (before spec folder write):
- candidates: `std_go_simpleredis_resp-commands`, `std_go_simpleredis_tcp-session`
- verdicts:
  - { deltaId: alloc-guards, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }

- folded std_go_simpleredis_resp-commands

FindSpecHost (archive):
- candidates: `std_go_simpleredis_resp-commands`, `std_go_simpleredis_tcp-session`
- verdicts:
  - { deltaId: std_go_simpleredis_resp-commands, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }

