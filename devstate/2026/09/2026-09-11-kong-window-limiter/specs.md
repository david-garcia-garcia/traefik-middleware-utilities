# Specs
change: add-ratelimit-sliding-window
- added std_go_ratelimit_sliding-take (new — family std_go_ratelimit; dest has reclaim and simpleredis only)
- added std_go_ratelimit_sync-flush (new — same family; not a fold into resp-commands)

FindSpecHost:
- sliding-take: new, spec-id std_go_ratelimit_sliding-take, confidence high, candidates [std_go_simpleredis_resp-commands, std_go_reclaim_value-lifecycle]
- sync-flush: new, spec-id std_go_ratelimit_sync-flush, confidence high, candidates [std_go_simpleredis_resp-commands]
