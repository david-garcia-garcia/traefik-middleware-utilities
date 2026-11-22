# Tasks

## 1. GoDoc

- [ ] 1.1 Extend `Memory.Allow` GoDoc in `tokenbucket/memory.go`: a ctx already done returns its error and charges nothing; HTTP mapping (499 on `Canceled`, no wait when `wait==0`) belongs to the middleware, citing traefik/traefik#14173; verify `go vet ./tokenbucket/` is clean and the comment ends with a period (`godot`)
- [ ] 1.2 Extend `Redis.Allow` GoDoc in `tokenbucket/redis.go`: a `context.Canceled` / `DeadlineExceeded` return does not prove the script did not run and the token may have been charged; same HTTP-mapping sentence and #14173 citation as 1.1; verify `go vet ./tokenbucket/` is clean, no production-logic diff besides comments (`git diff -- tokenbucket/memory.go tokenbucket/redis.go` is GoDoc only)

## 2. Memory canceled-context test

- [ ] 2.1 Add `tokenbucket/traefik_issue14173_test.go` with `countAdmits14173` (`t.Helper()`) and `TestTraefik14173_MemoryCanceledContext`: pre-canceled ctx, `errors.Is(err, context.Canceled)`, `!status.Allowed`, zero `Status` on error, then full burst afterwards via `status.Allowed` on `Allow(context.Background(), key)`; clock frozen, burst 3, rate `1.0/60`, `maxDelay` 0, TTL 120s (design D3–D4); do not copy other issues' tests; verify `go test -short -count=1 -run '^TestTraefik14173_MemoryCanceledContext$' ./tokenbucket/` passes and `TestStatus_ErrorReturnsTheZeroStatus` still passes

## 3. Live in-flight cancel test

- [ ] 3.1 Add `tokenbucket/traefik_issue14173_e2e_test.go` with `countingProxy14173`, `startCountingProxy14173`, `forwardHeld14173`, `liveKey14173`, `evalKey14173` (`t.Helper()` on test helpers) and `TestTraefik14173_RedisCanceledInFlight`: `runForEachLiveEngine` / `waitLiveClient` from `limiter_e2e_test.go`, warmup Allow, hold 1500ms, cancel after 200ms, assert `Canceled`, `!status.Allowed`, elapsed `< hold/2`, log `hmget last, tokens` after `hold+300ms` without `Fatal` on charged vs empty, no Lua `TIME` loop (design D3–D5); verify `go test -short -count=1 -run '^TestTraefik14173_RedisCanceledInFlight$' ./tokenbucket/` skips, and with `TOKENBUCKET_LIVE_REDIS=127.0.0.1:16379 TOKENBUCKET_LIVE_DRAGONFLY=127.0.0.1:16380` the same `-count=1 -run` (no `-short`) returns within half the hold on each engine

## 4. Integration

- [ ] 4.1 `go vet ./...` and `go test -short -count=1 ./...` green; confirm no shared helper file, no `TIME` busy loop, no production-logic change outside GoDoc, and no copy of other issues' tests (`grep` for unsuffixed `countAdmits`/`liveKey`/`countingProxy` in the two new files returns nothing)

## Workflow follow-up

- Usage packet `knowledge/devdocs/std_go_tokenbucket.md` is out of this apply's minimum; `opd-devdocsimpact` measures whether the in-flight charge belongs in Gotchas.
- Archive: no spec delta to fold (`skip_specs: true`; `devstate` specs.md is `none — no live contract`).
