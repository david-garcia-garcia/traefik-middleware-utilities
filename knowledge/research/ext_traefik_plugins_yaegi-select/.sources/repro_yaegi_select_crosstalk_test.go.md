---
url: simpleredis/repro_yaegi_select_crosstalk_test.go
title: simpleredis shared-select repro, interpreted vs compiled (this run)
fetched: 2026-11-21
authority: inference
---

Uncommitted repro files in the ticket worktree on 2026-11-21: `simpleredis/repro_yaegi_select_crosstalk_test.go` (harness) and `simpleredis/repro_yaegi_select_probe_test.go` (probes `SelectProbeWaitUntil`, `SelectProbeBorrowWait`, `SelectProbeTwoPools`, copied into a Yaegi v0.16.1 interpreter as a regular source file so they can call unexported functions).

Each probe runs compiled (control) and interpreted:

- `TestReproSelect_WaitUntil_Compiled` / `_Yaegi`: interpreted `waitUntil` 1 ms short waiters lost their own tick 6 and 21 times per run (of about 5–7k calls). Compiled: clean.
- `TestReproSelect_BorrowWait_Compiled` / `_Yaegi`: interpreted `borrowSocket` returned `(nil, nil)` 2 and 12 times per run. Compiled: clean.
- `TestReproSelect_TwoPools_Compiled` / `_Yaegi`: interpreted clients crossed in-use semaphores (freeA 30/32 with overFreesB 2; freeB 31/32 with overFreesA 1). Compiled: clean.

Separate saturated interpreted `Get` run on the same day: the test process panicked within 0.2 s with `reflect: call of reflect.Value.Field on zero Value`, Yaegi frame `simpleredis/resp.go:19:12` in `do`, nil conn.

Measurement on this run, not an upstream statement. Mechanism owner: `github.com/traefik/yaegi@v0.16.1:interp/run.go` (`.sources/run.go.md`).
