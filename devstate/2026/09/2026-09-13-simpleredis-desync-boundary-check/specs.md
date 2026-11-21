# Specs
change: simpleredis-leftover-reply-destroy
- fold std_go_simpleredis_tcp-session (leftover-bytes destroy / not pooled; high; candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_resp-decode, std_go_simpleredis_resp-commands)
- fold std_go_simpleredis_resp-commands (Get own-key after stray extra; high; candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session)
