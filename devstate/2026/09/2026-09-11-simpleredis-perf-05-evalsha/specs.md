# Specs
change: simpleredis-evalsha
- fold std_go_simpleredis_resp-commands  high  evalsha-inside-eval — EVALSHA-inside-Eval + NOSCRIPT fallback; Yaegi fallback; live Redis+Dragonfly SCRIPT FLUSH/EXISTS. Candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session, std_go_tokenbucket_lua-eval.
