---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/82498271f7f15b4aa128bb3e13fc015e40242e6f/pkg/lapi/zzz_poll_timer_test.go
title: crowdsec-bouncer-traefik-plugin no-select guard and Yaegi poll test (PR 208)
fetched: 2026-11-21
authority: source
ref: github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f:pkg/lapi/zzz_poll_timer_test.go
---

`Test_lapiHasNoSelectStatement`: `filepath.Glob("*.go")`, skips `_test.go`, `parser.ParseFile` each, `ast.Inspect` for `*ast.SelectStmt`; fails with the positions found. Comment: "yaegi v0.16.1 shares one select case slice per statement across every goroutine that runs it, so the loops of different Clients would wait on each other's channels."

`TestYaegi_pollTimer_manyLoopsStopIndependently`: `yaegitest.Run(t, yaegitest.GoPath(t), source)` where `source` is a `package main` that sets `GOMAXPROCS(2)`, starts 16 production poll timers at 1 ms via `lapi.StartPollTimerForTest`, waits for 5 runs each, stops the even ones, waits 20 ms, then requires each odd loop to advance 20 more runs (10 s deadline) and each stopped loop to stay frozen. Exits 1 on failure.

Compiled unit tests in the same file: re-arm after work, no overlapping work, stopping one timer does not stall another, stop during work does not re-arm, nil-safe idempotent stop, Client Sleep/Wake/Close stop and restart both timers.
