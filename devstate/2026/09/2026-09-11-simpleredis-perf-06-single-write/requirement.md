# Requirement
IssueKey: 2026-09-11-simpleredis-perf-06-single-write

## Problem
`writeCommand` encodes each RESP argument with string concatenation and three writer calls, then `Flush`. Under Yaegi that costs ~1,380 ns extra per command versus one scratch buffer and one `Write`. `dial` also allocates a 4 KB `bufio.Writer` per socket that becomes unused once the frame is a single write.

## Current (code)
- `simpleredis/simpleredis.go` `writeCommand` (lines 309–326) — `"*" + strconv.Itoa(...)` then per-arg `"$" + strconv.Itoa`, `Write(arg)`, `"\r\n"`, then `writer.Flush()`.
- `simpleredis/simpleredis.go` `do` (line 296) — `writeCommand(conn.writer, args)` then `readReply(conn.reader)`.
- `simpleredis/simpleredis.go` `pooledConn` (lines 41–46) — `netConn`, `*bufio.Reader`, `*bufio.Writer`; no scratch `buf`.
- `simpleredis/simpleredis.go` `dial` (lines 269–273) — `bufio.NewWriter(netConn)` (default 4096-byte write buffer).
- `simpleredis/simpleredis.go` `release` (lines 244–259) — idle cap `maxIdleConns` (8); no encode-buffer capacity trim.
- `simpleredis/simpleredis.go` exported verbs — `Get`, `MGet`, `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`; all go through `exec` → `do` → `writeCommand`.
- `simpleredis/simpleredis_test.go` `TestValueWithNewlinesSurvives` (line 338) — SET/GET round-trip of newline bytes.
- `simpleredis/simpleredis_test.go` `lastSetCommand` / `lastExpireCommand` / `lastEvalCommand` — fake-server argv assertions (parsed RESP, not a raw-byte golden).
- `simpleredis/bench_test.go` — not found on `origin/master`.
- `simpleredis/interpretedcost_test.go` — not found on `origin/master` (finding and `simpleredisfixes/test-07-hot-path-benchmarks.md` treat them as already added during the review; they exist only as untracked files in the dirty main checkout).
- `e2e/simpleredisprobe/plugin.go` — `ServeHTTP` runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval; Eval body `kongIncrbyExpireatScript` lists `KEYS[1]` and is Lua 5.1-safe (`table.maxn` not used).
- `docker-compose.yml` — `redis:7-alpine` at `redis:6379`; `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2` at `dragonfly:6379`; whoami `/redis` and `/dragonfly`.
- `scripts/integration-tests.Tests.ps1` — Pester asserts every verb header on `/redis` and `/dragonfly`.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — command shapes + dual-engine e2e; does not specify encode call count.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — stdlib-only session, no `go-redis`; does not mention `bufio.Writer`.
- `knowledge/devdocs/std_go_simpleredis.md` — Yaegi-safe client; Eval keys must be in `keys` (Dragonfly); no `table.maxn`.

## Desired
1. Encode one RESP array in a per-connection scratch `[]byte` with `append` + `strconv.AppendInt`, then one `netConn.Write`. Drop `bufio.Writer` from `pooledConn` / `dial`. Keep `bufio.Reader`.
2. Trim retained scratch capacity on `release` so a large `SET` does not pin a huge buffer on that idle conn for the process lifetime (finding example: drop if past 64 KB).
3. Wire bytes identical to today. Existing protocol tests stay green (`TestValueWithNewlinesSurvives`, argv assertions). Every existing verb still works live on Redis and Dragonfly: compose + Pester `/redis` and `/dragonfly`, `e2e/simpleredisprobe`. Do not change Eval script language (Lua 5.1-safe, KEYS required).
4. Keep compiled allocation guards `BenchmarkEncodeGet` / `BenchmarkEncodeEval` and Yaegi encode benches `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite`. Dest does not have those files yet — this change must land them (or equivalent) so they remain after the encoder switch. Production encode should match the single-write strategy those Yaegi benches measure.

## Affected
- `simpleredis/simpleredis.go` (`pooledConn`, `dial`, `do`, `writeCommand`, `release`)
- `simpleredis/simpleredis_test.go` if `writeCommand` signature or fake helpers change
- New or copied `simpleredis/bench_test.go` and `simpleredis/interpretedcost_test.go` (and any `yaegi_test.go` `testing.TB` helper widening they need)
- `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1` only if a verb or script would otherwise break (expected: run, not rewrite)
- Possibly `openspec/specs/std_go_simpleredis_resp-commands/spec.md` / tcp-session if encode/buffering is specified (today it is not)

## Out of scope
- perf-07 `readLine` / `ReadBytes`; dropping `bufio.Reader`
- perf-01/02 pool cap or I/O timeout; perf-03 idle reaper; perf-04 pipelining; perf-05 EVALSHA; feat-01 MSETEX; perf-08 `unsafe` zero-copy
- Importing `go-redis` or copying its `interface{}` writer
- New Redis verbs, TLS, Unix sockets, changing Init/Close/pool size
- Rewriting the probe Eval script or Dragonfly compose pin unless the encoder change forces it

## Unknowns
- Exact retained-buffer cap (finding says “for example, 64 KB”).
- Whether to copy the untracked review `bench_test.go` / `interpretedcost_test.go` as-is or rewrite them against the new `writeCommand` signature (`*bufio.Writer` today in those copies).
- Whether any spec leaf needs an encode-path requirement, or tests+e2e are enough.

## Tensions
- Finding “How to prove” talks only about existing protocol tests plus benches that are **not** on `origin/master`. Conductor hard-requires live Redis **and** Dragonfly e2e after the encoder change — follow the conductor.
- Finding presents those bench files as already in-tree; dest does not have them (test-07 “partially applied” in the review working copy only). Landing them is in Desired, not a silent extra product ask.
- Finding points at go-redis `internal/proto.Writer` as a pattern; tcp-session forbids importing a Redis client module — copy the idea, not the package.
