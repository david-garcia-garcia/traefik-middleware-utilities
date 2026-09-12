# Specs
change: bound-simpleredis-command-latency
- fold std_go_simpleredis_tcp-session (FindSpecHost: high; candidates std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands; small adjustment to defaults, overall budget, cancel)
- fold std_go_simpleredis_resp-commands (FindSpecHost: high; candidates std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session; *Context twins on existing verbs)
