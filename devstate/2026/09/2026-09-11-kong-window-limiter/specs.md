# Specs
change: add-ratelimit-sliding-window
- added std_go_windowcounter_sliding-take (new — family std_go_windowcounter; dest has reclaim and simpleredis only)
- added std_go_windowcounter_sync-flush (new — same family; not a fold into resp-commands)

FindSpecHost:
- sliding-take: new, spec-id std_go_windowcounter_sliding-take, confidence high, candidates [std_go_windowcounter_sliding-take, std_go_simpleredis_resp-commands, std_go_reclaim_value-lifecycle, std_go_simpleredis_tcp-session]
- sync-flush: new, spec-id std_go_windowcounter_sync-flush, confidence high, candidates [std_go_windowcounter_sync-flush, std_go_simpleredis_resp-commands, std_go_reclaim_value-lifecycle, std_go_windowcounter_sliding-take]
- archive 2026-09-11: same new verdicts; synced to openspec/specs/
- rename 2026-09-11: family `std_go_ratelimit` → `std_go_windowcounter` (package is the hit counter, not Traefik RateLimit)
