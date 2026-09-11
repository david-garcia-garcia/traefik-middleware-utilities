# Specs
change: simpleredis-idle-head-sweep
- fold std_go_simpleredis_tcp-session  high  idle-head-sweep — stale idle-head close on release; live Redis+Dragonfly idle-head tests in CI. Candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands.

