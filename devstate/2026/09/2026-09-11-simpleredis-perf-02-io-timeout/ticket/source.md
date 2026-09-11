# perf-02 — Fixed 1 s I/O timeout plus no pool cap fans out under Redis slowness

Local dump. Bound this finding only.

- Finding: `simpleredisfixes/perf-02-io-timeout-fan-out.md`
- Index: `simpleredisfixes/README.md`

## Finding

Every timing knob is a compile-time constant with no way for a caller to change it:

```
maxIdleConns = 8
idleTimeout  = 30 * time.Second
dialTimeout  = 2 * time.Second
ioTimeout    = 1 * time.Second
```

`do` applies `ioTimeout` as a single deadline covering the write and the read. A slow Redis makes each in-flight command occupy its connection for up to a full second. Combined with perf-01 (no cap on total connections, no wait queue) the two constants compound: while existing callers hold their sockets for a second, every newly arriving request dials a socket of its own.

`dialTimeout = 2 s` has the same problem from the other side: during a Redis outage, each request can spend 2 seconds in `net.Dialer.Dial` before failing.

## How to fix (this ticket)

Make the four values configurable while keeping the current numbers as defaults, so behaviour is unchanged unless a caller opts in.

- Extend `Init` or add an options struct (`InitWithOptions`, or exported fields set before first use) covering dial timeout, I/O timeout, idle timeout, idle cap, and the `poolSize`/`poolTimeout` from perf-01. Keep zero values meaning "default" so existing `Init(host, pass, database)` callers are untouched.
- Consider separate read and write deadlines rather than one combined `SetDeadline` (optional).
- Do not add retries around the timeout. `exec` already declines to retry `errTimeout`.
- Yaegi: keep the config surface as plain structs and primitives; no functional-option closures stored across the interpreter boundary.

## How to prove (finding)

A test with a fake server that never replies, asserting the command returns `redis:timeout` after the *configured* timeout rather than the constant. `TestIoTimeout` already covers the constant case and should keep passing on defaults. Finding also asks a test that a configured `poolSize` bounds live sockets while all of them sit in a stalled command.

## Caller bound

Implement configurable dial/io/idle timeouts and related Init/options as this finding's How to fix. Do not take over the wait-queue pool rewrite that belongs to perf-01; if this finding names poolSize/poolTimeout as config fields, add the knobs and document the tension rather than silently shipping perf-01's whole semaphore.

## Tests (required)

Tests MUST run against both Redis and Dragonfly. Both are supported backends. Fake-server tests are not a substitute for live proof of timeout/pool/verb behavior. Extend dest compose + Pester (`Test-Integration.ps1`, `/redis` and `/dragonfly`, `e2e/simpleredisprobe`) and/or live go tests so this ticket's new knobs are proven on both engines. Lua 5.1-safe. Dragonfly KEYS required. CI must exercise both backends.
