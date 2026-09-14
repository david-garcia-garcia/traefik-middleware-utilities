# Specs
change: simpleredis-unsupported-reply
- modified std_go_simpleredis_resp-commands
  FindSpecHost: fold, confidence high
  candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode
  why: small adjustment to exported error strings, array-element / nested scenarios, Eval return contract, and malformed vs unsupported split. Same leaf already owns those requirements.
