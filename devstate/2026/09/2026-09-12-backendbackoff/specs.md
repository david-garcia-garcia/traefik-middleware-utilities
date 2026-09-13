# Specs
change: backendbackoff-gate

FindSpecHost:
- delta `allow` → new `std_go_backendbackoff_allow` (high). Candidates: `std_go_tokenbucket_allow` (different clock, do not fold), `std_go_windowcounter_sliding-take` (hit counter, do not fold). Large new capability; family `std_go_backendbackoff` not on map.
- delta `cooldown` → new `std_go_backendbackoff_cooldown` (high). Candidates: none. Second clock of the same new family; not a small adjustment to tokenbucket.

- added std_go_backendbackoff_allow
- added std_go_backendbackoff_cooldown
