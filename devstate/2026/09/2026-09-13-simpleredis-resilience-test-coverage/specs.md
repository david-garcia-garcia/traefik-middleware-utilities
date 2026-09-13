# Specs
change: simpleredis-resilience-test-coverage
- fold std_go_simpleredis_tcp-session (chaos pool + lifecycle coverage)
- fold std_go_simpleredis_resp-decode (FuzzReadReply / FuzzParseLen)
- fold std_go_simpleredis_resp-commands (injection, concurrent MSetEX fallback, interpreted error paths)
