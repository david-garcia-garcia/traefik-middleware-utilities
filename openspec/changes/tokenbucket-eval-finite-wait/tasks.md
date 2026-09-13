## 1. Tests first (must FAIL on dest)

- [ ] 1.1 Add `tokenbucket/repro_eval_nan_wait_test.go` `TestRepro_EvalWaitNaNFailOpen` for waits `nan`, `+Inf`, `-Inf`, `inf` via fake Redis `arrayBulks("true", wait, "0")`
- [ ] 1.2 Adapt dest `Allow(ctx, key) (bool, time.Duration, error)`; assert non-nil `errEvalWait` and wait `0`; do not weaken to pass
- [ ] 1.3 Run `go test -short -count=1 -timeout 60s -run TestRepro_EvalWaitNaNFailOpen ./tokenbucket` and confirm FAIL (fail-open `allowed=true` `err=nil`)

## 2. Finite wait

- [ ] 2.1 After successful `ParseFloat` of Eval wait, if `math.IsNaN` or `math.IsInf` return `(false, 0, errEvalWait)`
- [ ] 2.2 Re-run `TestRepro_EvalWaitNaNFailOpen` and confirm PASS
- [ ] 2.3 Run `go test -short ./tokenbucket` and confirm existing tests stay green

## 3. Usage

- [ ] 3.1 Update `knowledge/devdocs/std_go_tokenbucket.md` Gotcha: non-finite Eval wait is `errEvalWait`, not admit/deny
