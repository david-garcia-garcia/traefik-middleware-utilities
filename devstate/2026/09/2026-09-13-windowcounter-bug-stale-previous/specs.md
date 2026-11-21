# Specs
change: windowcounter-stale-previous
- fold std_go_windowcounter_sync-flush (high) — candidates: std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take. Small adjustment: buffered Take GET previous when local_delta is 0; later Take after roll sees shared Redis previous; Peek stays skip-storm.
- fold std_go_windowcounter_sliding-take (high) — candidates: std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush. Small adjustment: buffered two-client dump at boundary denies at estimated 3; Peek may lag previous until Take.
