# Specs
change: simpleredis-peer-close-eof-redial
FindSpecHost:
- { deltaId: peer-closed-idle-retry, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands] }
- modified std_go_simpleredis_tcp-session

FindSpecHost (archive):
- candidates: `std_go_simpleredis_tcp-session`, `std_go_simpleredis_resp-commands`
- verdicts:
  - { deltaId: std_go_simpleredis_tcp-session, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands] }
