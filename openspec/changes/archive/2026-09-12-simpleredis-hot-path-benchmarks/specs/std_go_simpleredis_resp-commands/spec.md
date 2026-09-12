## ADDED Requirements

### Requirement: CI allocation guards fail on over-budget encode and decode
The compiled `go test` suite for SimpleRedis SHALL fail when client-side encode or decode of GET, EVAL, a bulk reply, a 10-slot array, an integer reply, or a 100 KB bulk exceeds the Go 1.21 `allocs/op` or `B/op` ceiling recorded in that test. Those guards SHALL run as compiled tests that measure `AllocsPerOp` and `AllocedBytesPerOp` without requiring `go test -bench`. They MUST NOT assert wall-clock `ns/op`. They MUST NOT dial live Redis or Dragonfly. Compose Redis (`redis:7-alpine`) and Dragonfly (`docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`) plus Pester `/redis` and `/dragonfly` SHALL keep proving Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval on both engines. Eval SHALL stay a Lua 5.1-safe script that lists its key in KEYS. CI MUST NOT drop or skip either engine’s tests.

#### Scenario: Over-budget allocs fail without bench flag
- **WHEN** `go test ./simpleredis/...` runs without `-bench`
- **AND** a client-side encode or decode loop reports `AllocsPerOp` or `AllocedBytesPerOp` above the Go 1.21 ceiling recorded in that test
- **THEN** that test fails

#### Scenario: 100 KB bulk decode is guarded
- **WHEN** the decode guard runs a canned `$102400` bulk GET of `100*1024` bytes
- **THEN** `AllocsPerOp` and `AllocedBytesPerOp` are compared to the Go 1.21 ceiling
- **AND** the fixture is not a live Redis or Dragonfly round-trip

#### Scenario: Live verb coverage stays on Redis and Dragonfly
- **WHEN** CI integration runs
- **THEN** Pester `GET /redis` and `GET /dragonfly` still assert Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval
- **AND** Eval uses a Lua 5.1-safe script with KEYS declared
- **AND** neither engine’s tests are skipped
