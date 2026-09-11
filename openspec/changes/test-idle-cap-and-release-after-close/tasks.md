## 1. Unit idle cap

- [ ] 1.1 Add a same-file hold fake in `simpleredis_test.go` (accept or reply gated on a channel; track currently-open accepted sockets). Do not add `bench_test.go`
- [ ] 1.2 Add a test that starts 16 Gets, releases the hold after ≥16 accepts, then asserts `len(idle) <= 8` and live fake sockets equal `len(idle)`
- [ ] 1.3 Raise `TestConcurrentCommandsStayWithinPool` above eight goroutines and assert `len(idle) <= 8` (drop the tautological eight-accept check)

## 2. Close and second closed check

- [ ] 2.1 Add a nil-checked same-package hook after idle-scan unlock and before the second `closed` check; name it for test; do not export; production path stays nil
- [ ] 2.2 Add a test that sets the hook to `Close` with empty idle so the second `closed` check returns `redis:unreachable` without dialing
- [ ] 2.3 Add an in-flight-Close test: hold a command, call `Close`, finish the command, assert idle empty, that socket closed, later Get is `redis:unreachable` with no new dial. Keep Close-then-Get redial coverage; drop the vacuous final idle assertion that never called `release`
- [ ] 2.4 Run `go test ./simpleredis/...` until the new and repaired tests pass

## 3. Live overlap on Redis and Dragonfly

- [ ] 3.1 In `scripts/integration-tests.Tests.ps1` Describe `simpleredis Yaegi e2e`, fire 16 parallel `Invoke-WebRequest` at `/redis` then at `/dragonfly`. After each batch, `docker compose exec redis redis-cli CLIENT LIST` and `docker compose exec redis redis-cli -h dragonfly CLIENT LIST` (bare, no `TYPE`/`ID`). Count LF lines minus the listing client; remaining ≤ 8. Do not stop `whoami-a` or `whoami-b`. Do not change the probe EVAL or compose wiring
- [ ] 3.2 Run `./Test-Integration.ps1` until Redis and Dragonfly Describes pass and reclaim stays green

## 4. Specs

- [ ] 4.1 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests
- [ ] 4.2 Run `openspec validate --change test-idle-cap-and-release-after-close --strict`
