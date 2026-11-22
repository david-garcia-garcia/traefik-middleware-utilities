# Standards

1. [hard] std_go_simpleredis — `simpleredis/traefik_issue12078_e2e_test.go:50` — `t.Logf` emits `Config.Pass` (`%q` on `tc.pass`), breaking the packet rule to never put Pass on log lines; sibling `TestLive_WrongPassword` logs only the error
   Fix: Drop the `t.Logf` call or log the subtest name and `err` without the password literal
   Quote:
   ```go
   t.Logf("Pass=%q: Get err=%v", tc.pass, err)
   ```
   Status: skipped
   Argument: Gotcha is slog/session logs (std_go_simpleredis.md:124); t.Logf is testing.T of dummy CI password "secret"; design copy-as-is of the live repro; not applied unattended.
2. [hard] Bound the ask — `knowledge/opendev.json:1` — the pinned product apply adds local OpenDev model/language settings and `.cursorignore` (`.cursorignore:1`) while proposal Impact names only `simpleredis/config.go` and `simpleredis/traefik_issue12078_e2e_test.go`
   Fix: Remove `knowledge/opendev.json` and `.cursorignore` from this branch, or land them in a separate change if the repo chooses to version them
   Status: skipped
   Argument: OpenDev catalog/ignore bootstrap written by opendev_settings_get; host tooling contract; Bound the ask vs AUTH ticket; not applied unattended.