# Specs
change: simpleredis-drop-select-under-yaegi
- fold std_go_simpleredis_tcp-session (confidence medium; MODIFIED 9 requirements: "in-use-turn channel" → "in-use-turn semaphore", pool-wait handoff/give-up; ADDED 2: session source contains no select statement, interpreted concurrency does not cross turns or deadlines; candidates: std_go_simpleredis_tcp-session, std_go_simpleredis_resp-commands, std_go_simpleredis_slog-events, std_go_simpleredis_live-e2e, std_go_ci_test-suites)
- fold std_go_simpleredis_resp-commands (confidence high; MODIFIED 1: Concurrent MSetEX unknown-command fallback stays balanced; candidates: std_go_simpleredis_resp-commands, std_go_simpleredis_tcp-session)
