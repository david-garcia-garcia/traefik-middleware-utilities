## 1. Tests that fail on dest

- [x] 1.1 Add package tests for rate `1e6/1.2` + maxDelay 1500ns and rate 999999 + maxDelay 1µs (adapt caller `TestRepro_MaxDelayTruncationFailOpen`; dest `Allow(ctx, key)` arity; do not weaken)
- [x] 1.2 Add package tests for maxDelay 0 + rate `1e12` and rate `1e-12` waitDuration overflow (adapt the two hunt repros; do not weaken)
- [x] 1.3 Run the new tests isolated and `go test -short -count=1 ./tokenbucket`; confirm they FAIL on dest production code

## 2. Microseconds admit

- [x] 2.1 `Memory.Allow` and `Redis.Allow`: allowed iff `waitMicro <= float64(maxDelay.Microseconds())`; Redis uses parsed `waitMicro` (no Duration for the bool)
- [x] 2.2 Delete `allowedFromWait` and named `waitDuration`; inline Duration conversion only for the second return; leave Lua and ARGV unchanged
- [x] 2.3 Re-run the new tests isolated and `go test -short -count=1 ./tokenbucket`; confirm they PASS

## 3. Catalog

- [x] 3.1 Update `knowledge/devdocs/std_go_tokenbucket.md` gotcha: Go maps allowed from wait microseconds vs maxDelay microseconds
- [x] 3.2 `openspec validate tokenbucket-allow-microseconds-admit --type change --strict`
