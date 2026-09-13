# Specs
change: simpleredis-timeout-connect-per-command
FindSpecHost:
- deltaId: timeout-second-connection-per-command
  fold: std_go_simpleredis_tcp-session
  confidence: high
  candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands]
- added std_go_simpleredis_tcp-session (fold; ADDED requirement, no new leaf)
