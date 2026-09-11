# Specs
change: windowcounter-peek
- fold std_go_windowcounter_sliding-take  high  peek-observe — Peek without increment, agree with Take, sliding cooldown, Peek Redis errors, Yaegi Peek+Take. Candidates: std_go_windowcounter_sliding-take, std_go_windowcounter_sync-flush.
- fold std_go_windowcounter_sync-flush  high  peek-sync — exact GET every Peek; buffered skip-storm no GET every Peek; live Peek+Take. Candidates: std_go_windowcounter_sync-flush, std_go_windowcounter_sliding-take.
