# SimpleRedis

## Language

**SimpleRedis**:
A stdlib pooled TCP RESP client (`New(Config)`, `Get`, `MGet`, `Set` with EX, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, `Close`). Every verb takes `context.Context` as its first argument. `New` copies `Config` and does not dial; the first command dials. `New` returns `ErrMaxIdleConnsAbovePoolSize` and no client when, after defaults, `MaxIdleConns` is above `PoolSize`. Pool, timeout, and retry knobs live on `Config` and freeze at `New`. A caller with no deadline passes `context.Background()`.
This package exists because `go-redis` cannot run under Traefik's Yaegi at any symbol set: without `useUnsafe` it fails on `import "unsafe"`, with it current v9 fails on `unsafe.String`, and the last v9 that imports cleanly segfaults the host process on its first PING (`knowledge/research/ext_go-redis_yaegi-compatibility/`).
_Avoid_: `go-redis`, miniredis, TLS, Unix sockets, renaming the package to `redis`

**Reply boundary**:
The point after one complete RESP value where the connection reader has no leftover bytes. A socket that is not on a reply boundary is destroyed, not returned to idle.
_Avoid_: draining leftover bytes to resynchronise; treating a clean parse as reusable without an empty reader

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis`. Callers match errors with `errors.Is` against the exported sentinels (`ErrMiss`, `ErrUnreachable`, `ErrPoolWait`, `ErrUnsupportedReply`, …) or `IsMiss` / `IsUnreachable` / `IsPoolWait`. The string consts (`redis:miss`, `redis:unreachable`, `redis:unsupported-reply`, …) stay for display and legacy text matching. The copied sources are Apache-2.0 (`LICENSE`).

## How to use

- Call `simpleredis.New(simpleredis.Config{Host: host})` once before concurrent use and handle the error. After defaults, `MaxIdleConns` above `PoolSize` (including default idle 8 against a smaller `PoolSize`) is `ErrMaxIdleConnsAbovePoolSize` and no client. A trim below `PoolSize` is valid. Set pool, timeout, and retry knobs on `Config` (`MaxRetries` 0 at New means 1 extra retry; `-1` turns extra retries or backoff off). After `New` those knobs do not change. Set `Pass` and `Database` on `Config` when AUTH or SELECT is needed. Set `Logger` to a `*slog.Logger` when Traefik or the process should see `simpleredis_*` lines; nil at `New` becomes a discard logger so call sites never nil-check. Never put `Pass`, Redis key names, or values on log lines. One hop’s worst-case wait is `CommandTimeout` (900ms at zero Config); `MaxRetries` bounds attempts, not wall time, so raising it does not raise that ceiling. Eval NOSCRIPT (EVALSHA then EVAL) and MSetEX unknown-command (native then Eval) are separate hops, each with that budget; do not treat one public verb as one stacked deadline. Every verb takes a context first (`Get(ctx, name)`). Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline.
- Do not dial in Traefik `New`. Call `simpleredis.New` there; first command in `ServeHTTP` after Redis is up (`Set`, `Get`, `Incr`, `Eval`, or `MSetEX`).
- Call `MSetEX(ctx, names, values, seconds)` or `MSetEXAt(ctx, names, values, unixSeconds)` for many keys with one TTL. Do not MSET then EXPIRE. Match integer `0` as `redis:issue?`.
- Match AUTH-class Redis errors as `redis:noauth` (`ErrNoAuth`). Match miss with `IsMiss`, unreachable with `IsUnreachable`, pool saturation with `IsPoolWait`. Do not type-assert `net.Error` (Yaegi).
- Prove with `go test -short ./simpleredis/...` (unit + Yaegi fake, including fuzz seeds, RESP injection, concurrent MSetEX fallback, and interpreted error paths). Chaos pool and lifecycle stress skip under `-short`; run `go test ./simpleredis/` (no `-short`) for those. Under `-race`, concurrent MSetEX fallback stays on (4 workers × 8 calls). Skip only `TestYaegiErrorpath_PoolWait`: the detector reports `WARNING: DATA RACE` inside `github.com/traefik/yaegi/interp` (`_select` / `itype.refType`), not this client. Other interpreted error paths stay on. Live Redis/Dragonfly is `*_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`): dest engines for pool wait, SELECT 99, and CLIENT KILL; `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` for WRONGPASS. Yaegi live covers every public verb. Traefik e2e is `./Test-Integration.ps1` (`scripts/integration-tests.simpleredis.Tests.ps1`): one file; `-Engine redis` or `-Engine dragonfly` sets `INTEGRATION_ENGINE` and drives `/<engine>/<verb>` (status + body). Exact `/redis` and `/dragonfly` are health Set+Get. Helpers live in `scripts/integration-tests.utils/`. CI splits reclaim (`Integration Tests`) from those two engine jobs. Allocation guards are `TestAlloc*` functions that call `testing.Benchmark` with `ReportAllocs` and fail on over-budget allocs/op or B/op; they do not need `-bench`. They skip when the race detector is on.
- Call `Eval(ctx, script, digest, keys, args)` with the Lua body and `ScriptSHA1Hex(script)` (compute once at init for a reused script). Eval MUST NOT hash. It sends EVALSHA of that digest; on NOSCRIPT it falls back once to EVAL so the engine stores the script. Do not SCRIPT LOAD at `New`. A digest that does not match the body pays EVAL every call. Return values are a flat array of bulk strings or integers; wrap each Lua slot with `tostring` (or return numbers). Top-level Lua `false`/`nil` is one nil slot with a nil error, not `redis:miss`. Nested tables and `{err=...}` inside an array are `redis:unsupported-reply` and close the socket. Traefik `/eval` takes that digest as query `digest`.

## Pattern snippet

```go
client, err := simpleredis.New(simpleredis.Config{Host: "redis:6379"})
if err != nil {
	return err
}
ctx := context.Background()
if err := client.Set(ctx, "k", []byte("v"), 60); err != nil {
	return err
}
got, err := client.Get(ctx, "k")
if err != nil {
	return err
}
n, err := client.Incr(ctx, "counter")
if err != nil {
	return err
}
if err := client.MSetEX(ctx, []string{"a", "b"}, [][]byte{[]byte("1"), []byte("2")}, 60); err != nil {
	return err
}
```

## Key files

- `simpleredis/simpleredis.go` — client, New, Close; nil `Config.Logger` becomes a discard logger
- `simpleredis/config.go` — freeze-at-New knobs
- `simpleredis/pool.go` — in-use turns, unused sockets, dial
- `simpleredis/commands.go` — thin verbs
- `simpleredis/commands_exec.go` — exec, retry
- `simpleredis/commands_eval.go` — Eval (EVALSHA, NOSCRIPT → EVAL)
- `simpleredis/commands_msetex.go` — MSetEX / MSetEXAt, capability cache
- `simpleredis/resp.go` — RESP codec
- `simpleredis/peer_drop_all_test.go` — sequential Gets after every accepted socket is dropped
- `simpleredis/pool_e2e_test.go` — live pool wait, SELECT 99, WRONGPASS, CLIENT KILL recover
- `simpleredis/commands_e2e_test.go`, `commands_eval_e2e_test.go`, `commands_msetex_e2e_test.go` — live verbs
- `simpleredis/simpleredis_e2e_test.go` — skip/fail harness
- `simpleredis/yaegi_e2e_test.go` — interpreted live verbs
- `simpleredis/bench_test.go` — encode/decode benches and CI alloc guards
- `simpleredis/interpretedcost_test.go` — asserting Yaegi conversion matrix, copy-vs-unsafe benches, and Yaegi encode cost
- `simpleredis/yaegi_test.go` — interpreter New/Get/Set/Del/Incr/Eval/MSetEX; live LiveVerbs is the full public set
- `simpleredis/yaegi_defer_test.go` — interpreted defer runs on panic (evidence that overturns PR #29)
- `simpleredis/panic_safety_test.go` — recovered panic inside `do` returns the turn and closes the socket
- `simpleredis/yaegi_errorpath_test.go` — interpreted dial-retry, stall timeout, cancel-mid-command, pool wait, truncated bulk
- `simpleredis/chaos_pool_test.go` — chaos mix: no cross-key Get, at-rest turns, `OverFrees() == 0`, settled open sockets
- `simpleredis/lifecycle_test.go` — New/use/Close cycles and Close during held Gets
- `simpleredis/commands_msetex_concurrent_test.go` — concurrent unknown-command MSetEX fallback
- `simpleredis/resp_injection_test.go` — CRLF and inline PING round-trip as data
- `simpleredis/BUGS.md` — hunt record: handshake matcher under Yaegi; interpreted defer-on-panic proof (`yaegi_defer_test.go`); desync eviction. Rejected rows name the locking test
- `simpleredis/slog_test.go` — capturing slog handler, secrets, alloc guards
- `e2e/simpleredisprobe/plugin.go` — Traefik local plugin (`Host`, optional `Password`, `Database`, `DropHost`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md`, `openspec/specs/std_go_simpleredis_resp-commands/spec.md`, `openspec/specs/std_go_simpleredis_slog-events/spec.md`
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — ReadSlice decode, escaping `+`/`:` copies, `parseLen`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md`

## Gotchas

- `New` does not dial. A refusing host is fine until the first command.
- After `Close`, later commands return `redis:unreachable` and do not redial. Closed-client unreachable is not retried. Call `Close` while a command is in flight: that command may finish; its socket is closed on release, not returned to idle.
- Live cap is `Config.PoolSize` (`PoolSize()`, const default 8; unused plus in-flight). `New` builds the in-use-turn channel. After `New` a `PoolSize` write does not resize. A waiter past `PoolSize` waits `PoolTimeout` (default 200ms), then `redis:unreachable`. Idle trim is `MaxIdleConns` (default 8). Pool wait is not retried. An extra return of an in-use turn when the channel is already full does not hang: it is dropped and `OverFrees()` increments. Dest used to park that goroutine forever. Correct accounting stays at `OverFrees() == 0`.
- Do not treat `len(idleConns)` plus `len(inUseTurns)` as a live-socket count. The two reads are not atomic, and `release` publishes into `idleConns` before `freeInUseTurn`, so a releasing socket is counted in both (up to 2× PoolSize). Server-side peak open also over-counts because the serve goroutine exits after the client's `Close`. Assert only at-rest turns (`len(inUseTurns) == cap`) and settled server open ≤ PoolSize. Do not assert peak live sockets.
- Borrow sweeps idle sockets older than `IdleTimeout` (default 30s) and closes them, including a stale head behind a younger tail that is reused. `New` starts no reaper; a fully quiet client keeps those sockets until `Close`.
- A client that did not come from `New` (`&SimpleRedis{}`) fails the first command immediately with `redis:unreachable` and is not retried.
- Yaegi tests copy non-test sources into GOPATH with stdlib only (`useunsafe` false).
- Do not adopt `unsafe` zero-copy between `string` and `[]byte`. Compiled it saves a little on Eval encode; under Yaegi (the Traefik plugin path) the legacy struct-header trick is a net loss versus inline `[]byte(s)`. Keep `[]byte(...)` / `string(...)` in session source.
- Do not import `unsafe` in session source. Do not set `useUnsafe` on the plugin manifest or compose `settings.useunsafe`. Traefik registers unsafe symbols only when **both** the manifest and operator settings are true; manifest true and settings false refuses to load the plugin (`knowledge/research/ext_traefik_plugins_useunsafe/`).
- CI fails if the Yaegi conversion matrix cells change, if a non-test session file imports `unsafe` or `"C"`, or if probe/compose `useUnsafe` becomes true. Named copy-vs-unsafe benches reproduce ns/op (`go test -run XXX -bench`); they do not fail `go test` without `-bench`.
- Yaegi v0.16.1 does not export `unsafe.Slice` / `unsafe.String` / `StringData` / `SliceData` even when unsafe symbols are registered (`knowledge/research/ext_traefik_plugins_yaegi-unsafe/`).
- `Incr` / `IncrBy` do not refresh TTL. `Expire` / `ExpireAt` integer `0` is success, not `redis:miss`. `MSetEX` / `MSetEXAt` integer `0` (and any integer other than `1`) is `redis:issue?`.
- Eval scripts that touch keys must list those keys in `keys` (Dragonfly rejects undeclared keys). Do not use `table.maxn` (Dragonfly Lua 5.4).
- `MSetEX` / `MSetEXAt` reject empty or mismatched slices and more than 1024 pairs with `redis:issue?` before dial. Zero or negative TTL is passed through, same as `Set`. Clustered engines need every key in one hash slot (hash tags); the client does not hash-tag or split.
- Redis 7 and Dragonfly have no native `MSETEX`. The first call sends native, then caches Lua `Eval` (names in KEYS, values then `EX`/`EXAT` then TTL in ARGV). Redis 8.4+ / Valkey 9.1+ stay on native after a successful `MSETEX`. A later `ERR unknown command` recaches Lua and `Eval`s that call. Native MSETEX and that Eval are separate hops; Eval starts with a full command budget. Past `MSetEXAt` may return no error while a later `Get` is `redis:miss`.
- Eval sends EVALSHA of the caller digest. Compute `ScriptSHA1Hex` once next to a reused script const. Do not keep a client digest table. SCRIPT FLUSH or a restart yields NOSCRIPT; Eval then sends EVAL once so the engine stores the script. A wrong digest repeats that fallback. Callers still pass the body for that miss path. EVALSHA and EVAL are separate hops; each binds its own overall deadline so EVAL still has a full command budget. Do not share remaining time across hops (that can starve EVAL).
- Every verb uses go-redis-shaped command retry (`MaxRetries` / `MinRetryBackoff` / `MaxRetryBackoff` on `Config`; `MaxRetries` 0 at New means 1 extra retry; backoff `0` is 8ms / 512ms; `-1` is off). Each hop (`exec`) has an overall deadline of `CommandTimeout` (900ms at zero Config). Eval NOSCRIPT and MSetEX unknown-command are two or three hops; each hop binds a fresh budget. When that instant is sooner than the caller, `exec` binds it with `context.WithDeadline` (same as `net.Dialer`). Remaining time is shared by every attempt and by the dial, AUTH, SELECT, and command I/O inside them, so retries stop at whichever comes first: the attempt count or the budget. The overall deadline closes the socket so a drip peer cannot pin a turn. Retry `redis:unreachable` (including a fresh dial) and LOADING/READONLY/MASTERDOWN/CLUSTERDOWN/TRYAGAIN / max-clients replies. Do not retry `redis:timeout`, a pool-wait timeout, a client that did not come from `New`, a cancelled context, `redis:noauth`, or other Redis `-ERR` replies including handshake AUTH/SELECT failures. INCR/INCRBY/EVAL can double-apply after a lost reply; that is accepted. `ErrPoolWait` and a client-not-from-New both wrap `ErrUnreachable`, so `IsUnreachable` is true; use `IsPoolWait` to shed load. Do not rewrite retry onto `errors.Is(err, ErrUnreachable)` alone.
- There are two timeout knobs and each bounds something on its own: `DialTimeout` (200ms) caps one TCP dial attempt via `net.Dialer.Timeout`, `CommandTimeout` (900ms) caps the whole command. There is no third knob, and none of them is a multiplicand of another — read the ceiling off `CommandTimeout`, do not compute it. Dest exported an `IOTimeout` that bounded no single operation once the socket deadline became the budget remainder; it was removed rather than left as a knob you had to put in a formula.
- The socket `SetDeadline` is whatever is left of the hop's budget, so a reply that keeps arriving is not cut off for its size — that is what made a large value permanently unreadable, since the decoder accepts bulks to `maxBulkLength` (64 MiB) that the old 100ms per-operation cap could never carry, and `redis:timeout` is not retried. Measured at zero Config: 8 MiB streamed over ~535ms returns intact, and a peer that goes quiet mid-reply ends at 900ms with `redis:timeout`. Do not add a second per-`Read` deadline next to it: two bounds on one socket can only disagree, the shorter one wins for the wrong reason, and the `stallConn` wrapper that did this was rejected as too much machinery (a full hand-written `net.Conn`, because Yaegi does not promote methods from an embedded interface). The accepted price is that a peer going quiet mid-reply spends the whole budget. To tighten that ceiling, lower `CommandTimeout`.
- RESP2 null array `*-1` is `redis:issue?` (this client has no BLPOP/MULTI/EXEC). Null bulk `$-1` is a nil slot from decode. Get maps a one-slot nil reply to `redis:miss`. Eval `return false` / `return nil` is that same wire and MUST NOT be `redis:miss`. Empty bulk `$0` is a non-nil empty slice, not a miss. Do not treat null array and null bulk as the same.
- A well-framed type this decoder does not decode (nested array, `-` inside an array, HTTP-shaped or RESP3 type byte) is `redis:unsupported-reply`, not `redis:issue?`. The socket is discarded. Do not retry it.
- After one complete reply, leftover unread RESP means the peer is ahead of the protocol. That command still returns its decoded value; the socket is discarded, not drained, and not pooled. Do not drain leftover bytes to resynchronise. A compliant peer leaves the reader empty, so sequential Gets still reuse one connection. Unread leftover before the next write on the same socket (AUTH then SELECT) is `redis:unreachable` and is not written. Same text as a short bulk read: the socket is unusable and the command is not. Do not tidy that to `redis:issue?` — that token is not retried. Handshake leftover still does not open a second dial (`handshakeFailed`). Those two leftover cases emit `simpleredis_socket_poisoned` and `simpleredis_auth_leftover` at Warn.
- `Config.Logger` is frozen at `New`. Nil becomes a discard logger. Call sites log inline `simpleredis_*` strings (no message constants). Error means an operator must act (`simpleredis_panic`, `simpleredis_noauth`, `simpleredis_over_free`). There is no Info tier. Never log `Pass`, Redis keys, or values. That bans the peer's AUTH error text too: Redis 7.4 answers a nopass default user with `ERR AUTH <password> called without any password configured`, which is not an AUTH-class prefix, so `simpleredis_handshake_failed` carries `verb` (`AUTH` / `SELECT`) and not the reply. The returned error still carries the full text for the caller.
- `simpleredis_dial` carries `reason`: `idle_miss` when the unused list had nothing to reuse, `skip_idle` when this command already failed I/O on a socket it took from that list and is bypassing it. A burst of `skip_idle` is the idle-vintage-drop signature (peer restart, failover, `CLIENT KILL`); a burst of `idle_miss` is pool pressure. Do not read one as the other.
- A server-closed unused socket younger than IdleTimeout (default 30s) is borrowed and maps I/O to `redis:unreachable` (`io.EOF`). Remaining attempts of that command skip idle and dial, so sequential Gets after a full idle vintage drop succeed at default MaxRetries. MaxRetries `-1` still fails that command (one send). Leftover unused corpses stay parked; a later sequential command spends one then force-dials. Do not prove peer close by closing the client fd (`SetDeadline` then fails with `os.ErrClosed` and never reaches `ioError`).
- Why that shape exists. The unused pool is there so a sequential burst on one client pays one dial plus one AUTH/SELECT instead of one per command. The bill is that a Redis restart, a failover, or `CLIENT KILL` of every accepted socket leaves the parked sockets dead but younger than `IdleTimeout`, so the sweep does not touch them and a borrow hands out a corpse; before skip-idle, one command burned every attempt on corpses and returned `redis:unreachable` against a healthy peer (measured 4 failed requests at defaults). go-redis never hands out that socket because `connCheck` asks the fd first (`syscall.Conn` → `RawConn.Read` → non-blocking `syscall.Recvfrom` of one byte with `MSG_PEEK|MSG_DONTWAIT`; `EAGAIN`/`EWOULDBLOCK` healthy, zero bytes dead, a byte that is there unexpected — and peeked, so the reply still has to be pulled from the socket afterwards). **Do not port that probe.** Traefik registers `syscall` symbols only when `useUnsafe` is true on both the manifest and the operator's static config, and manifest-true with operator-false makes Traefik refuse to load the plugin outright — not something a plugin other operators install can require. The probe is also Unix-only, and Yaegi does not evaluate `//go:build`, so go-redis's own `conn_check.go` / `conn_check_dummy.go` split silently resolves to the no-op. Detecting the dead socket by using it is what is left (`knowledge/research/ext_go-redis_pool_conn-check/` for the probe itself, `ext_traefik_plugins_useunsafe/`, `ext_traefik_plugins_yaegi-build-constraints/`).
- Do not use `//go:build` to vary a non-test session file by OS. Yaegi v0.16.1 evaluates only the legacy `// +build` form, which `gofmt` strips at `go >= 1.17`, so every variant loads and filename sort order silently picks the live body. Filename GOOS/GOARCH suffixes (`foo_linux.go`) **are** honored by `skipFile` and are the only reliable mechanism. Session source currently carries no build constraints at all; keep it that way (`knowledge/research/ext_traefik_plugins_yaegi-build-constraints/`).
- Match AUTH-class prefixes (`NOAUTH`, `WRONGPASS`, `NOPERM`, `ERR Client sent AUTH`) as `redis:noauth`. Redis 7.4 `AUTH` against a nopass default user returns `ERR AUTH <password> called without any password configured…`, which is **not** that prefix — it is a plain error. Dragonfly without `--requirepass` accepts any `AUTH` password (`OK`). Wrong-password live proof needs `requirepass` on both engines.
- Empty password skips AUTH; empty database skips SELECT. When both are set, AUTH runs before SELECT.
- `SELECT` out of range is `ERR DB index is out of range`, not `redis:noauth`. A handshake AUTH or SELECT error closes the socket, is not pooled, and is not retried. Peer-close is the unreachable sentinel; AUTH-class is `ErrNoAuth`. Those values match `errors.Is` / `IsUnreachable` under Yaegi (the session does not wrap them in a package-local type).
- Probe success labels on `/redis` and `/dragonfly` stay host-only (empty password and database). Failure routes `/redis-wrong-password` `/dragonfly-wrong-password` set `Password`; `/redis-database-99` `/dragonfly-database-99` set `Database`.
- Probe verb paths (`/<engine>/<verb>`) are terminal: HTTP 200 + Redis body or 502 + `err.Error()`. They do not forward to next. Set `CommandTimeout` to 3s on the probe clients so a 500ms TIME-wait Eval is not bound by the default 900ms budget.
- Interpreted `defer` runs under Yaegi v0.16.1 (this module's pin, and Traefik v3.7.11's pin) when the panic is an explicit interpreted panic, the interpreter's own `errors.As` panic on a package-local struct, or a nil-map write. Yaegi converts that panic into an `Eval` error rather than propagating a Go panic outward, but it still unwinds interpreted frames and runs their defers. Do not re-derive PR #29's assumption that a deferred `release` cannot save an in-use turn. Proof: `simpleredis/yaegi_defer_test.go`.

