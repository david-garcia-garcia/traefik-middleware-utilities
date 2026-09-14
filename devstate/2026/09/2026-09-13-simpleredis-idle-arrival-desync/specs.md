# Specs
change: none
- skip_specs: true (no spec-level behavior change)
- considered fold into `std_go_simpleredis_tcp-session` leftover exclusion and `std_go_simpleredis_resp-commands` Get exclusion — not written; dest already excludes kernel-buffer idle arrival
- the `simpleredis-idle-arrival-desync` change folder was withdrawn: no product code ships, so a live change on master would stay in-flight forever. Its decision record now lives in `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md`.
