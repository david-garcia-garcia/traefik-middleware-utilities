# Specs
change: simpleredis-readslice-decode

FindSpecHost (before each folder):

- { deltaId: resp-decode, new, spec-id: std_go_simpleredis_resp-decode, confidence: high, candidates: [std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode] }
- { deltaId: live-verbs, fold, spec-id: std_go_simpleredis_resp-commands, confidence: high, candidates: [std_go_simpleredis_resp-commands] }

- added std_go_simpleredis_resp-decode
- modified std_go_simpleredis_resp-commands
