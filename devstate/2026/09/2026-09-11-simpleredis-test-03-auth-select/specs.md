# Specs
change: simpleredis-test-03-auth-select
- modified std_go_simpleredis_tcp-session (fold, high) — handshake AUTH/SELECT failure closes, is not pooled, maps AUTH-class to redis:noauth, no redial; fake plus live skip-if-unset
- modified std_go_simpleredis_resp-commands (fold, high) — probe Password/Database; Pester wrong-password and SELECT 99 routes; keep no-password /redis /dragonfly SHALL
