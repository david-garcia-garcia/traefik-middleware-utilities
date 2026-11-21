# Specs
change: simpleredis-tls-acl
- modified std_go_simpleredis_tcp-session — fold, high (MODIFIED: New records settings and does not dial; First command dials TCP; AUTH and SELECT run once per dial)
- added std_go_simpleredis_tls-handshake — new, medium (ADDED: A TLS rejection returns redis:tls; redis:tls is not retried and is not unreachable; A slow or silent TLS peer stays redis:unreachable; TLS failures are classified without net.Error or AlertError; A TLS session works under Yaegi). Candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_slog-events
- modified std_go_simpleredis_slog-events — fold, high (ADDED: A TLS failure is Error at the site that saw it; MODIFIED: A peer reply is not published)
- modified std_go_simpleredis_live-e2e — fold, high (ADDED: Live TLS and ACL user on Redis and Dragonfly; Yaegi live TLS and ACL user on both engines)
