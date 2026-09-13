# Specs
change: simpleredis-handshake-no-redial
- fold std_go_simpleredis_tcp-session (high) candidates: std_go_simpleredis_tcp-session — handshake MUST NOT second TCP; add AUTH EOF, SELECT EOF, AUTH LOADING, AUTH max-clients scenarios
