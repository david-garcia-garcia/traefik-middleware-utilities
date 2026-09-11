# Devdocs impact
change: add-leakybucket

## Units
- Leaky bucket — subsystem — `leakybucket/`; specs `std_go_leakybucket_pour`, `std_go_leakybucket_sync-flush`

## Findings
- [x] stale-usage  Key files — `std_go_leakybucket` still pointed at `openspec/changes/add-leakybucket/`; siblings name live `openspec/specs/`
- [x] stale-usage  Gotchas — `std_go_leakybucket` omitted the memory map cap (65536) this apply added
