## 1. Constructor

- [ ] 1.1 Add `Config` with `Grace` next to `Table` in `reclaim/table.go`. Replace `NewTable(grace)` with `New(Config)` that copies `Grace` (negative → `DefaultGrace`, zero stays 0). No setter.
- [ ] 1.2 Replace every `NewTable(...)` in `reclaim/table_test.go` and `reclaim/yaegi_test.go` with `New(Config{Grace: ...})`.
- [ ] 1.3 Add a test that writing `Config.Grace` after `New` does not change the table's grace.

## 2. Drop the process table

- [ ] 2.1 Delete `reclaim/default.go`.
- [ ] 2.2 Remove `TestDefault_*`. Keep sharing coverage as two `Open`s on one `New` table. Package `Reset` / `ResetWith` call sites become `tab.Reset()` on an instance.
- [ ] 2.3 Run `go test -count=1 -timeout 60s ./reclaim/`.

## 3. Callers and usage

- [ ] 3.1 In `e2e/reclaimprobe/plugin.go`, hold a package-level table `New(Config{Grace: DefaultGrace})` and call `table.Open` from plugin `New`.
- [ ] 3.2 Update `README.md`, `knowledge/devdocs/std_go_reclaim.md` (drop Language **Default**; production is a caller-owned table), and `knowledge/devdocs/std_go_backendbackoff.md` (`reclaim.Open` → store in a table the caller owns).
- [ ] 3.3 Run `go test -count=1 -timeout 60s ./reclaim/` again after the probe compile still type-checks against the new API.
