# Specs
change: windowcounter-reject-fractional-window
- fold std_go_windowcounter_sliding-take (high) — small adjustment: explicit reject for non-whole-second windows (1500ms), not only sub-second. Candidates: std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush.
