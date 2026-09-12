# Specs
change: simpleredis-exec-panic-releases-token
- fold std_go_simpleredis_tcp-session (FindSpecHost: high; candidates std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode; panic-unwind is a small adjustment to live-cap/release, not a new family)
- archive FindSpecHost: fold std_go_simpleredis_tcp-session (high; candidates std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode, std_go_simpleredis_resp-commands, std_go_simpleredis_live-e2e)
