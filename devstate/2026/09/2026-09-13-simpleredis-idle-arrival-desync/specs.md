# Specs
change: simpleredis-idle-arrival-desync
- skip_specs: true (no spec-level behavior change)
- considered fold into `std_go_simpleredis_tcp-session` leftover exclusion and `std_go_simpleredis_resp-commands` Get exclusion — not written; dest already excludes kernel-buffer idle arrival
