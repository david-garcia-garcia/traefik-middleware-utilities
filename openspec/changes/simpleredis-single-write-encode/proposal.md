## Why

Under Yaegi, dest `writeCommand` pays ~1,380 ns extra per command by concatenating RESP headers and issuing several `bufio.Writer` calls plus `Flush`. Traefik plugins run that path on every Get/Set/Eval. Encode into one scratch buffer and one `net.Conn.Write` so interpreted plugins stop paying that tax, without changing wire bytes or dropping Redis or Dragonfly proof.

## What Changes

- Encode each RESP array in a per-connection scratch `[]byte` (`append` + `strconv.AppendInt`) and one `netConn.Write`. Drop `bufio.Writer` from `pooledConn` / `dial`. Keep `bufio.Reader`.
- Trim retained scratch on `release` when `cap` exceeds 64 KiB so a large SET cannot pin that idle conn for the process lifetime.
- Wire bytes identical to dest today. Compiled golden of encoded GET. Existing protocol tests stay green.
- Tests MUST run against both Redis and Dragonfly (both supported backends). Prove every existing verb still works live on both engines after the encoder change: compose + Pester `/redis` and `/dragonfly`, `e2e/simpleredisprobe`. Lua 5.1-safe. Dragonfly KEYS required. Do not rewrite the probe Eval script or the Dragonfly compose pin unless the encoder change forces it.
- Land compiled allocation guards `BenchmarkEncodeGet` / `BenchmarkEncodeEval` and Yaegi encode benches `BenchmarkYaegiEncodeBufio` / `BenchmarkYaegiEncodeSingleWrite` rewritten against the new encoder (do not copy the untracked review files as-is).
- No new verbs. Do not import `go-redis`. Do not change Init/Close/pool size, TLS, or Unix sockets.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-commands`: encoded RESP bytes MUST match dest framing; prove every existing verb live on Redis and Dragonfly after the encoder change (compose + Pester `/redis` `/dragonfly`, `e2e/simpleredisprobe`); keep Lua 5.1-safe Eval with KEYS; land compiled and Yaegi encode benches.
- `std_go_simpleredis_tcp-session`: idle reusable conns MUST drop an encode scratch whose `cap` exceeds 64 KiB; session source stays stdlib-only (no `go-redis`).

## Impact

- `simpleredis/simpleredis.go` (`pooledConn`, `dial`, `do`, `writeCommand`, `release`).
- `simpleredis/simpleredis_test.go` (wire golden, idle-buffer trim, existing protocol tests).
- New `simpleredis/bench_test.go` and Yaegi encodeprobe benches; widen `writeGopathFile` to `testing.TB`.
- Run, do not rewrite: `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1`.
- Neighbors (`tokenbucket`, `windowcounter`) stay green through exported verbs.
- Main specs `std_go_simpleredis_resp-commands` and `std_go_simpleredis_tcp-session` after archive.
