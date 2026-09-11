# Specs
change: simpleredis-exec-pipeline

FindSpecHost:
- delta ExecPipeline/cap/per-element/Yaegi/Traefik-header → fold std_go_simpleredis_resp-commands confidence high; candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session
- delta pipeline-retry → fold std_go_simpleredis_tcp-session confidence high; candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands

- modified std_go_simpleredis_resp-commands
- modified std_go_simpleredis_tcp-session
