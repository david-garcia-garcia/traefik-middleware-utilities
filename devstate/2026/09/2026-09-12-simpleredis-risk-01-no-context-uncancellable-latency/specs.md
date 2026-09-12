# Specs
change: bound-simpleredis-command-latency
- fold std_go_simpleredis_tcp-session (FindSpecHost: high; candidates std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands; small adjustment to defaults, overall budget, cancel)
- fold std_go_simpleredis_resp-commands (FindSpecHost: high; candidates std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session; *Context twins on existing verbs)
archive FindSpecHost (2026-09-12):
- fold std_go_simpleredis_tcp-session (high; candidates std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands, std_go_simpleredis_resp-decode, std_go_simpleredis_live-e2e)
- fold std_go_simpleredis_resp-commands (high; candidates std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session, std_go_simpleredis_live-e2e)
