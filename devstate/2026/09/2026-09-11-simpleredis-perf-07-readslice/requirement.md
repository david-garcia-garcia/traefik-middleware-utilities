# Requirement
IssueKey: 2026-09-11-simpleredis-perf-07-readslice

## Problem
`readLine` uses `bufio.Reader.ReadBytes`, so every RESP line allocates a copy. Most copies are discarded after reading the type byte or length. In a Traefik process doing many `MGet`/`Eval` array replies that is avoidable GC pressure. `ReadSlice` is the intended fix, but its buffer view is invalid after the next read, so escaping `+`/`:` payloads must be copied, and lines longer than the 4096-byte buffer must handle `bufio.ErrBufferFull`.

## Current (code)
- `simpleredis/simpleredis.go:408-417` `readLine` — `reader.ReadBytes('\n')`, strip CRLF; no `ReadSlice`, no `ErrBufferFull` path.
- `simpleredis/simpleredis.go:338-340` — `+`/`:` return `line[1:]` without copying (safe today because `ReadBytes` already owns the slice).
- `simpleredis/simpleredis.go:376-377` — array `:`/`+` elements store `head[1:]` into `values[i]` with no copy.
- `simpleredis/simpleredis.go:353` and `:393` — array count and bulk length via `strconv.Atoi(string(...))`.
- `simpleredis/simpleredis.go:389-405` `readBulk` — `make([]byte, length+2)` then `io.ReadFull`; payload returned to caller.
- `simpleredis/simpleredis.go` — exported `Get`, `MGet`, `Incr`/`IncrBy`, `Eval` (and Set/Del/Expire/ExpireAt) all decode through `readReply`/`readLine`.
- `simpleredis/simpleredis_test.go` — compiled fake RESP; no copy-after-next-read test; no line longer than 4096 bytes / `ErrBufferFull`.
- `simpleredis/bench_test.go` — `not found` (`BenchmarkDecodeBulk` / `BenchmarkDecodeArray10` / `BenchmarkDecodeInteger` named by the finding are not on dest).
- `e2e/simpleredisprobe/plugin.go` — each request runs Get, MGet, Incr, Eval (and Set/Del/Expire/ExpireAt) against `Config.Host`.
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379`; `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`; whoami routes `/redis` and `/dragonfly`.
- `scripts/integration-tests.Tests.ps1` — Pester asserts verb headers on `/redis` and `/dragonfly`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — Get/MGet/Incr/Eval wire plus Traefik e2e on both engines; Eval scripts Lua 5.1-safe with keys in KEYS.
- `knowledge/devdocs/std_go_simpleredis.md` — same Eval KEYS / no `table.maxn` gotcha; no ReadSlice decode notes.

## Desired
- Switch `readLine` to `ReadSlice('\n')`. On `bufio.ErrBufferFull`, accumulate into a fresh buffer and keep reading until a full line (go-redis `internal/proto.Reader.readLine` pattern).
- Copy `line[1:]` (and array `head[1:]`) wherever a `+` or `:` payload escapes to the caller or into `values[i]`. Header-only uses (type byte, length parse) must not keep the slice across a later read.
- Parse bulk/array lengths from bytes (`parseLen`) instead of `strconv.Atoi(string(...))`.
- Leave `readBulk`'s `make([]byte, length+2)` as-is.
- Unit-test copy-on-escape: a `+`/`:` value must still be correct after a subsequent read on the same connection.
- Unit-test a status or error line longer than 4096 bytes (`ErrBufferFull`).
- After the change, live Get/MGet/Incr/Eval replies stay correct on both Redis and Dragonfly via compose + Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe`. Tests must run against both backends.
- Eval scripts stay Lua 5.1-safe and list touched keys in KEYS (Dragonfly).

## Affected
- `simpleredis/simpleredis.go` (`readLine`, `readReply`, `readBulk`, new `parseLen`)
- `simpleredis/simpleredis_test.go` (copy-on-escape + `ErrBufferFull`)
- `simpleredis/bench_test.go` if created or if test-07 lands first (allocation guards named by the finding)
- Existing e2e already covers live verbs: `e2e/simpleredisprobe/plugin.go`, `scripts/integration-tests.Tests.ps1`, `docker-compose.yml` (must still pass; no new engines)

## Out of scope
- Shrinking `readBulk` to `make([]byte, length)` plus a two-byte CRLF discard (finding: cosmetic).
- perf-06 encode path, perf-08 `unsafe` zero-copy, pool cap, pipelining, EVALSHA, MSETEX.
- Adding Dragonfly/Redis compose services (already on dest).
- Changing Init/Close/pool, AUTH/SELECT, or exported error strings.
- Importing `go-redis` or miniredis.

## Unknowns
- Exact go-redis `readLine` fallback loop for `ErrBufferFull` (not in this tree; finding names `internal/proto.Reader.readLine`).
- Whether `bench_test.go` will exist before this change (test-07 is a separate finding, status not applied). If absent, allocation proof is new benches in this change or a later test-07 ticket.
- Whether `-` error payloads that escape via `replyError` → `string(message)` also need an explicit copy before the next read (finding names `+`/`:`; `string()` already copies).

## Tensions
- Finding tells implementers to assert new numbers on `BenchmarkDecode*` in `simpleredis/bench_test.go`; dest has no such file (test-07 not applied). Proof of allocation drop is therefore either this change adding those benches or waiting on test-07.
- Finding frames the job as decode allocs, not live backends; conductor requires unit tests for copy-on-escape/`ErrBufferFull` and live Get/MGet/Incr/Eval correctness on Redis and Dragonfly — follow the conductor addendum.
