# Specs
change: simpleredis-handshake-sentinel-match
- fold std_go_simpleredis_tcp-session (high) — handshake THEN matcher + no type wrap; candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands
- fold std_go_simpleredis_resp-commands (high) — interpreted handshake-error matching; candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session
