## 1. SimpleRedis package

- [x] 1.1 Copy `simpleredis.go` from crowdsec-bouncer `pkg/simpleredis` @ `6548da47` into `simpleredis/` (`package simpleredis`; this module’s import prefix only; stdlib imports; no `go.mod` Redis require)
- [x] 1.2 Copy `simpleredis_test.go` (in-process fake TCP) and adapt the import path
- [x] 1.3 Add `simpleredis/LICENSE` (Apache-2.0 plus Containous SAS / Traefik Labs copyrights from the source appendix)
- [x] 1.4 Update README Libraries (title SimpleRedis, Status Current), Layout (`simpleredis/`), and Tests (`go test ./simpleredis/...`); note the copied client is Apache-2.0
- [x] 1.5 Run `go test ./simpleredis/...` on this host until passing

## 2. Yaegi interpreter tests

- [x] 2.1 Add `simpleredis/yaegi_test.go` mirroring `reclaim/yaegi_test.go` (GOPATH copy of non-test sources, interp stdlib only, `useunsafe` false; compiled test owns the fake TCP listener; interpreted probe Init/Get/Set/Del)
- [x] 2.2 Run `go test ./simpleredis/...` including the Yaegi tests until passing

## 3. Yaegi e2e harness

- [x] 3.1 Add `e2e/simpleredisprobe/` (`package simpleredisprobe`, module `github.com/david-garcia-garcia/simpleredisprobe`, replace to repo root, `.traefik.yml`, `Config`/`CreateConfig`/`New`; `New` Inits only; `ServeHTTP` SET+GET and `X-SimpleRedis-Value`)
- [x] 3.2 Extend `docker-compose.yml`: `redis:7-alpine` at `redis:6379` no password, localPlugins `simpleredisprobe` `useunsafe: false`, GOPATH mount for the probe, `whoami-redis` on `/redis`. Keep project `reclaim-e2e`, `reclaim-e2e-traefik`, ports 8000/8080, routes `/a` `/b`
- [x] 3.3 Extend `Test-Integration.ps1` with Redis + `/redis` waits; add a Pester Describe for SimpleRedis that does not stop `whoami-a`/`whoami-b`; CI failure logs dump `redis` and `whoami-redis`
- [x] 3.4 Run `./Test-Integration.ps1` on this host with Docker until passing (reclaim Describe still green)

## 4. Specs on disk for apply

- [x] 4.1 Confirm change specs `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` match the copied API (no `pkg/simpleredis` paths)
- [x] 4.2 Run `openspec validate --change add-simpleredis --strict`
