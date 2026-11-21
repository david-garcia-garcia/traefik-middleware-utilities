# Specs
change: simpleredis-structured-logging

FindSpecHost:
- { deltaId: logger-config, fold, spec-id: std_go_simpleredis_tcp-session, confidence: high, candidates: [std_go_simpleredis_tcp-session] }
- { deltaId: slog-events, new, spec-id: std_go_simpleredis_slog-events, confidence: high, candidates: [std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands, std_go_simpleredis_slog-events] }

- added std_go_simpleredis_slog-events
- modified std_go_simpleredis_tcp-session
