# Specs
change: simpleredis-bulk-trailer

verdicts:
  - { deltaId: bulk-trailer-read, fold, spec-id: std_go_simpleredis_resp-decode, confidence: high, candidates: [std_go_simpleredis_resp-decode, std_go_simpleredis_resp-commands] }
  - { deltaId: bulk-trailer-not-pooled, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session] }

- modified std_go_simpleredis_resp-decode
- modified std_go_simpleredis_resp-commands
