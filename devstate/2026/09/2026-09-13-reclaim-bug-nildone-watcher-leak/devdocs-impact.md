# Devdocs impact
change: reclaim-nildone-watcher-exit

## Units
- Reclaim table — subsystem — `reclaim/table.go` / `knowledge/devdocs/std_go_reclaim.md`

## Findings
- [x] stale-usage  nil-Done watcher lifetime — `std_go_reclaim` Gotchas now say the watcher exits when the incarnation ends without drop
