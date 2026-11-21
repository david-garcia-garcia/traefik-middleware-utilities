# Yaegi select statement

How Yaegi v0.16.1 (Traefik v3.7.11 pin, this module's `go.mod`) runs a `select` statement when several goroutines execute the same statement, and what that does to interpreted code.

## One case slice per statement

`_select` in `interp/run.go` builds the statement's `n.exec` closure once for the `select` node. It allocates `cases := make([]reflect.SelectCase, nbClause+1)` there, outside `n.exec`. Every frame (every goroutine) that runs that statement uses the same slice.

On each run, `n.exec`:

- writes `cases[nbClause] = f.done` (only the read of `f.done` is under `f.mutex.RLock`)
- writes `cases[i].Chan` (and `cases[i].Send` for send clauses) from the current frame's values
- calls `reflect.Select(cases)`

There is no lock around the writes plus the `reflect.Select` call. The received value is stored with `assignedValues[j](f).Set(v)` into the current frame, whatever channel `j` held at the time.

Source: `github.com/traefik/yaegi@v0.16.1:interp/run.go` `func _select` (line 3749); extract `.sources/run.go.md`.

## Cross-talk between goroutines

Inference from the source above: two goroutines running the same `select` with different channels can overwrite each other's entries between the writes and `reflect.Select`. One goroutine then waits on, receives from, or sends to the other goroutine's channels (timer, `ctx.Done()`, semaphore, result channel), and stores the value in its own variables. The race is per statement, not per channel: different instances (different clients, pools, middlewares) that share code collide.

Reproduced on this repo (2026-11-21, interpreted vs compiled control):

- `waitUntil` with 1 ms waiters: short waiters lost their own tick 6 and 21 times per run (of about 5–7k calls).
- `borrowSocket`: returned `(nil, nil)` 2 and 12 times per run.
- Two clients: in-use semaphores crossed (A free 30/32 with B over-freed 2; B free 31/32 with A over-freed 1).
- Saturated interpreted `Get` panicked the test process within 0.2 s: `reflect: call of reflect.Value.Field on zero Value`, Yaegi frame `simpleredis/resp.go:19:12` in `do`, nil conn.

Compiled controls passed. Source (inference, measurement on this run): `simpleredis/repro_yaegi_select_crosstalk_test.go` `TestReproSelect_{WaitUntil,BorrowWait,TwoPools}_{Compiled,Yaegi}` with probes in `simpleredis/repro_yaegi_select_probe_test.go`; extract `.sources/repro_yaegi_select_crosstalk_test.go.md`.

## Plain receive and send are not shared

`recv`, `recv2` and `send` in the same file do not use a per-statement slice. Non-cancellable mode calls `Value.Recv` / `Value.Send` directly. Cancellable mode tries `TryRecv` / `TrySend`, then calls `reflect.Select` on a fresh slice literal `[]reflect.SelectCase{done, {...}}` built inside `n.exec` on each run.

Source: `github.com/traefik/yaegi@v0.16.1:interp/run.go` `func recv` (3517), `func recv2` (3593), `func send` (3685); extract `.sources/run.go.md`.

## Downstream fix pattern

`crowdsec-bouncer-traefik-plugin` PR #208 removed `select` from `pkg/lapi`:

- `pkg/lapi/poll_timer.go`: ticker + `select` loops replaced by `time.AfterFunc` that re-arms after work; stop sets a flag under a mutex and stops the timer. The doc comment states the same mechanism (one case list per statement shared across goroutines).
- `pkg/lapi/zzz_poll_timer_test.go`: `Test_lapiHasNoSelectStatement` parses every non-test `*.go` in the package with `go/parser` and fails on any `*ast.SelectStmt`. `TestYaegi_pollTimer_manyLoopsStopIndependently` runs 16 production poll timers through the `yaegi` binary (`pkg/yaegitest`), stops half, and fails if a stopped loop keeps running or a live loop stalls.
- `pkg/yaegitest.Run` skips when `yaegi` is not on `PATH`.
- CI `.github/workflows/main.yml` installs Yaegi v0.16.1 and runs `make yaegi_test` (`yaegi test -v .`, root package only). A separate `race` job runs `go test -race -count=1 -p 1 ./pkg/...` and does not install Yaegi.

Source: `github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin@82498271f7f15b4aa128bb3e13fc015e40242e6f` paths above and `Makefile`; extracts `.sources/poll_timer.go.md`, `.sources/zzz_poll_timer_test.go.md`, `.sources/interpreter.go.md`, `.sources/main.yml.md`, `.sources/Makefile.md`.

Related: `ext_traefik_plugins_yaegi-afterfunc/` (`context.AfterFunc` is available in Yaegi v0.16.1).
