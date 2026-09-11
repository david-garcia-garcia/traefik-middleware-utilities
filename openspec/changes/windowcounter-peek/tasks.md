## 1. Peek on the limiter

- [x] 1.1 Extract shared window-start, Redis keys, weight, and TTL helper; Take and Peek both call it
- [x] 1.2 Add `Peek(key, limit, window) (bool, float64, error)`: exact mode two GET via `getCount`; no INCR; no EXPIRE
- [x] 1.3 Buffered Peek under `l.mu`: seed GET + expireAt on first sight of the current-window key; return `redisKnown+localDelta` without increment; do not call `windowLocked`; previous window via `bufferedCountLocked`
- [x] 1.4 Keep `Allow` as Take

## 2. Unit tests

- [x] 2.1 Add a test-only GET call counter on `testFakeRedis`
- [x] 2.2 Unit: N Peeks then Take sees count 1; Peek agrees with Take at a frozen clock before the increment
- [x] 2.3 Unit: after Takes deny, Peek stays denied, then becomes allowed when the clock advances per the formula (not `2×window`)
- [x] 2.4 Unit: buffered skip storm does not GET every Peek; exact Peek GETs current and previous every call

## 3. Live and Yaegi

- [x] 3.1 Live table-driven Redis + Dragonfly: Peek then Take (no increment); skip on `-short` or missing addrs
- [x] 3.2 Yaegi: interpreted probe calls Peek and Take (fake + live GOPATH copies, stdlib only)
- [x] 3.3 Run `go test ./windowcounter/...` until passing (unit always; live when engines are up)

## 4. Usage packet and validate

- [x] 4.1 Update `knowledge/devdocs/std_go_windowcounter.md`: Language Peek; How to use Peek vs Take; buffered hot path
- [x] 4.2 Run `openspec validate --change windowcounter-peek --strict`
