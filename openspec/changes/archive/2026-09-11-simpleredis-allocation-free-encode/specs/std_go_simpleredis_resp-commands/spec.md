## ADDED Requirements

### Requirement: Encoded RESP bytes match dest framing on both backends
The client SHALL encode each command as the same RESP array of bulk strings dest currently produces (`*` count, per-arg `$` length, payload, CRLF), including native `MSETEX` argv from `MSetEX` / `MSetEXAt`. Tests MUST run against both Redis and Dragonfly (both supported backends). After the encoder change, compose plus Pester `/redis` and `/dragonfly` plus `e2e/simpleredisprobe` SHALL prove every existing verb still works live on both engines: Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt. Eval SHALL remain a Lua 5.1-safe script that lists its key in KEYS. The probe Eval body and the Dragonfly compose pin MUST NOT be rewritten unless the encoder change forces it.

#### Scenario: Encoded GET matches dest framing
- **WHEN** GET is encoded for key `session:9f2c1ab4-user-token`
- **THEN** the wire bytes equal dest framing `*2\r\n$3\r\nGET\r\n$27\r\nsession:9f2c1ab4-user-token\r\n`

#### Scenario: Encoded MSETEX matches dest framing
- **WHEN** native MSETEX is encoded for two pairs `a`/`1` and `b`/`2` with EX 60
- **THEN** the wire bytes equal dest framing `*8\r\n$6\r\nMSETEX\r\n$1\r\n2\r\n$1\r\na\r\n$1\r\n1\r\n$1\r\nb\r\n$1\r\n2\r\n$2\r\nEX\r\n$2\r\n60\r\n`

#### Scenario: Every verb still works live on Redis and Dragonfly
- **WHEN** a request is made on `/redis` (Redis `redis:7-alpine` at `redis:6379`) and on `/dragonfly` (Dragonfly `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`) after the encoder change
- **THEN** each response includes headers for Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEX-TTL that show those commands succeeded
- **AND** Eval used a Lua 5.1-safe script that lists `KEYS[1]`
- **AND** neither route’s tests stop `whoami-a` or `whoami-b`

#### Scenario: Newline value still round-trips
- **WHEN** Set stores a value that contains newline bytes
- **AND** Get is called for that key
- **THEN** Get returns those exact bytes

### Requirement: RESP framing does not allocate, measured compiled
Compiled tests SHALL include `BenchmarkEncodeGet`, `BenchmarkEncodeEval`, `BenchmarkEncodeMSetEX`, and `BenchmarkEncodeSet100KB` that encode on the production encoder. The framing itself SHALL NOT allocate: `BenchmarkEncodeGet`, `BenchmarkEncodeMSetEX`, and `BenchmarkEncodeSet100KB` SHALL each report 0 allocs/op, and any allocation `BenchmarkEncodeEval` reports SHALL come from building that command's argv, not from framing it. Length headers MUST be written with `strconv.AppendInt` into a connection-owned buffer and MUST NOT be built as a concatenated string (`"$" + strconv.Itoa(n) + "\r\n"`). That concatenation is the entire allocation this requirement exists to forbid; it was three allocations on a compiled GET and it is now none.

The encoder SHALL write through a `bufio.Writer` owned by the pooled connection. Framing a whole command into a growable per-connection scratch and issuing a single `net.Conn.Write` is **rejected** and MUST NOT be reintroduced. It was implemented and measured: `bufio` already coalesces a small command into exactly one write, and small commands (counters, TTLs, short Lua) are this session's workload, so the single write bought no syscall there. What it did cost is a full extra copy of every payload and a 64 KiB idle-retention cap that had to be specified and tested so one large SET could not pin a scratch on a parked socket. Measured compiled, a 100 KiB SET frames in 65 ns through `bufio` against 1590 ns through the scratch, because `bufio` hands a large slice straight to the socket instead of copying it.

This requirement is anchored on compiled numbers because the deployment this session is tuned for is compiled into Traefik; an interpreted measurement MUST NOT be the justification for an encoder shape. Interpreter tests SHALL keep `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite`, which record that the rejected shape is the cheaper one interpreted (measured on a GET: 6330 ns / 166 allocs through `bufio` against 2599 ns / 69 allocs through the scratch, because every call crossing the Yaegi boundary is expensive and the shipped shape makes five per argument). That is a cost this session knowingly accepts: the package must stay loadable as a Yaegi plugin for operators who install it that way, and the e2e suite uses Yaegi plugins because rebuilding Traefik costs minutes, but neither makes interpreted speed the optimisation target. Those two benches MUST NOT be deleted and that result MUST NOT be presented as anything other than what it is.

#### Scenario: Compiled encode benches exist
- **WHEN** package `simpleredis` encode benches run
- **THEN** `BenchmarkEncodeGet`, `BenchmarkEncodeEval`, `BenchmarkEncodeMSetEX`, and `BenchmarkEncodeSet100KB` execute against the production encoder

#### Scenario: Framing a command allocates nothing
- **WHEN** `BenchmarkEncodeGet` and `BenchmarkEncodeMSetEX` run compiled
- **THEN** each reports 0 allocs/op
- **AND** any allocation `BenchmarkEncodeEval` reports comes from building the argv, not from framing it

#### Scenario: Framing a 100 KiB payload allocates nothing either
- **WHEN** `BenchmarkEncodeSet100KB` runs compiled
- **THEN** it reports 0 allocs/op
- **AND** the payload is not copied into an encode scratch first

#### Scenario: Single-write framing stays rejected
- **WHEN** the production encoder is read
- **THEN** it writes through the pooled connection's `bufio.Writer`
- **AND** there is no growable per-connection encode scratch
- **AND** there is no idle-retention cap for such a scratch

#### Scenario: Interpreted encode cost is recorded, not acted on
- **WHEN** `BenchmarkYaegiEncodeBufio` and `BenchmarkYaegiEncodeSingleWrite` run
- **THEN** both execute and the rejected single-write shape is the cheaper interpreted one
- **AND** that result does not change the compiled encoder shape
