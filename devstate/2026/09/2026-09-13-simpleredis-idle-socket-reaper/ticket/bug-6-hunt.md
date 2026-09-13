# BUG-6 hunt excerpt (not product source)

Untracked in the caller checkout; do not commit `simpleredis/PRODUCTION-BUGS.md` or `simpleredis/bugs_production_test.go` on this branch.

- Hunt report: `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` (BUG-6). Sibling defect is BUG-1.
- Tagged reproduction: `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` (`//go:build simpleredis_bugs`).
- Function name in that file: `TestBugIdleSocketsAreNeverReapedWithoutTraffic` (caller invoke used `TestBugIdleSocketsAreNeverReaped`).
- Measured on dest-shaped code: `IdleTimeout` 50 ms, 500 ms of silence, idle list 4, server-side open sockets 4.
- Hunt SHA named in that report: `master` @ `751500e` (this run’s dest HEAD is recorded on `handoff.yaml` `src`).
