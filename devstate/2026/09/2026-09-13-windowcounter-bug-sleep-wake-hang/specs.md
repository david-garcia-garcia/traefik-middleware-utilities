# Specs
change: windowcounter-sleep-wins-wake
- fold std_go_windowcounter_sync-flush (confidence high; candidates std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take)
- archive-sync fold std_go_windowcounter_sync-flush (MODIFIED Sleep Wake Close reclaim the flush ticker)
