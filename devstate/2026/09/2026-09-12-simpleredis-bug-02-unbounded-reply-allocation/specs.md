# Specs
change: simpleredis-reply-alloc-caps
- fold std_go_simpleredis_resp-decode (high) — candidates: std_go_simpleredis_resp-decode, std_go_simpleredis_resp-commands — small ceiling in front of existing make+ReadFull
- fold std_go_simpleredis_resp-commands (high) — candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session — over-cap is redis:issue?, truncated under-cap stays I/O
