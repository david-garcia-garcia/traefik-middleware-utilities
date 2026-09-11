# Devdocs impact
change: explicit-reclaim-lifecycle-hooks

## Units
- Reclaim table — subsystem — `reclaim/table.go` (`knowledge/devdocs/std_go_reclaim.md`)
- Hooks — pattern — `reclaim.Hooks` on `Open` (`knowledge/devdocs/std_go_reclaim.md`)

## Findings
- [x] language-gap  Close — `std_go_reclaim` has Sleep and Wake Language, no Close term
- [x] stale-usage  Close — `std_go_reclaim` How-to still said `Close` as a method; Gotchas named Wake stall, not Close-hook block
