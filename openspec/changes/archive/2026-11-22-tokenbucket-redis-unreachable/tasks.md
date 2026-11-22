# Tasks

## 1. GoDoc on Redis.Allow

- [x] 1.1 Extend the GoDoc on `tokenbucket.Redis.Allow` (`tokenbucket/redis.go`) so an outage returns `simpleredis.ErrUnreachable` (or `ErrTimeout`) with `allowed=false`, fail-open or fail-closed is the middleware's decision, and the comment cites https://github.com/traefik/traefik/issues/12091; do not change the `Allow` body (keep `return Status{}, err` on Eval failure); verify `gofmt` and `go vet ./tokenbucket/` are clean and `git diff` of the function body is empty

## 2. Dedicated unreachable test

- [x] 2.1 Add self-contained `tokenbucket/traefik_issue12091_test.go` with `TestTraefik12091_RedisUnreachable`: `simpleredis.New` at `127.0.0.1:1` with `MaxRetries: -1`, `NewRedis`, dest signature `(Status, error)`, `errors.Is(err, simpleredis.ErrUnreachable)` and `!status.Allowed`; helpers only in this file and suffixed `12091` (call `t.Helper()`); do not rewrite `TestRedis_Unreachable` or `TestStatus_ErrorReturnsTheZeroStatus`; do not add a live or `ErrTimeout` case; verify `go test -short -count=1 -timeout 2m -run '^TestTraefik12091_RedisUnreachable$' ./tokenbucket/` passes

## 3. Usage matching

- [x] 3.1 In `knowledge/devdocs/std_go_tokenbucket.md` How to use, replace "Match Redis errors by `Error()` text" with `errors.Is` / `IsUnreachable` against `ErrUnreachable` and `ErrTimeout` (display text stays `redis:unreachable`); leave the fail-closed snippet that returns `err != nil`; do not edit `std_go_windowcounter.md`; verify the How to use bullet names `errors.Is` and `IsUnreachable` and no longer instructs matching by `Error()` text

## 4. Integration

- [x] 4.1 `go vet ./...` and `go test -short -count=1 ./...` green; existing `Error()` tests still pass; `git diff` of `simpleredis/` is empty
- [x] 4.2 Live tests of the touched package with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379` `TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` (and the SimpleRedis AUTH pair if that package's tests run in the same command): `go test -count=1 -timeout 5m ./tokenbucket/`; skip only when those engines are not up, then rely on CI `e2e-redis` / `e2e-dragonfly`

## Workflow follow-up

- Archive folds the MODIFIED delta into `openspec/specs/std_go_tokenbucket_allow/spec.md`.
- CI GitHub (golangci-lint v1.63.4 with unused/unparam/godot/thelper, Unit, Unit race, e2e-redis, e2e-dragonfly) green on the PR.
