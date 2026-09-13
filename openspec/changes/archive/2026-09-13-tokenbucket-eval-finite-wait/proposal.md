## Why

`Redis.Allow` treats a 3-field Eval wait of `nan` / `+Inf` / `-Inf` / `inf` as a successful parse and admits (`allowed=true`, `err=nil`). Callers see a successful consume. A garbage string already becomes `errEvalWait`; non-finite numbers must use that same sentinel.

## What Changes

- After a successful `ParseFloat` of the Eval wait field, require a finite number (`math.IsNaN` / `math.IsInf`). Otherwise return `errEvalWait`. Do not admit. Do not return `allowed=false` with `err=nil`. Do not add a second error type.
- Land unit tests first that fail on dest for those four waits (fake Redis 3-field reply), then the finite check, then PASS.
- Bug 1’s microsecond admit/refund compare does not replace this (`-Inf <= maxDelayMicro` is true).

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: a 3-field Eval wait that is not a finite number is `errEvalWait`, not admit or deny.

## Impact

- `tokenbucket/redis.go` `Allow` after `ParseFloat` of `values[1]`.
- `tokenbucket/repro_eval_nan_wait_test.go` (adapt dest three-return `Allow`).
- `openspec/specs/std_go_tokenbucket_allow/spec.md` after archive.
- `knowledge/devdocs/std_go_tokenbucket.md` Gotcha for non-finite wait.
- Do not change `Memory.Allow`, Lua `allowScript`, or other tokenbucket bugs.
