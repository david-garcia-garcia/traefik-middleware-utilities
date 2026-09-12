## 1. Sweep on borrow

- [ ] 1.1 In `takeIdleConn`, sweep the whole idle list: keep still-young sockets, collect stale, reuse the newest survivor (LIFO); leave `borrow` closing stale after unlock
- [ ] 1.2 Do not start a goroutine in `New`; do not peel heads in `release`

## 2. Proof

- [ ] 2.1 Add a fake test: two overlapping Gets so two sockets idle, age only the head `lastUsed`, one Get, `waitOpenSocketsEqual` for the closed head, `connections()` still 2
- [ ] 2.2 Assert `runtime.NumGoroutine()` is unchanged across `New`
- [ ] 2.3 Run `go test -short ./simpleredis/...` until the new tests and existing idle tests pass

## 3. Usage and spec

- [ ] 3.1 After the sweep lands, add a usage gotcha on `knowledge/devdocs/std_go_simpleredis.md` that borrow closes stale idle heads
- [ ] 3.2 Run `openspec validate --change simpleredis-idle-borrow-sweep --strict`
