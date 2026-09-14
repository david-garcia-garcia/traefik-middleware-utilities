# Specs
change: simpleredis-iotimeout-stall

FindSpecHost:
- deltaId: stall-iotimeout
  fold|new: fold
  spec-id: std_go_simpleredis_tcp-session
  confidence: high
  candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode]

- modified std_go_simpleredis_tcp-session
