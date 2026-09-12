# Requirement
IssueKey: 2026-09-11-simpleredis-test-04-truncated-bulk

## Problem
`readBulk`'s short-read (`io.ReadFull` failure) and malformed bulk-header branches have no tests. `clean == false` is what stops a misaligned socket from returning to the idle pool. Without that assertion, a truncated Redis/Dragonfly reply can leak one request's bulk payload into the next request's Get/MGet.

## Current (code)
- Short read: `simpleredis/simpleredis.go:400-403` (`io.ReadFull` into `length+2`; return `err` on failure). No test executes this branch.
- Non-`$` bulk head: `simpleredis/simpleredis.go:390-392` → `errIssue`. No test.
- Unparseable bulk length: `simpleredis/simpleredis.go:394-396` → `errIssue`. No test.
- Legitimate miss (`length < 0`): `simpleredis/simpleredis.go:397-399` → `errMiss`. Covered by Get-miss tests in `simpleredis/simpleredis_test.go`.
- `readReply` maps non-miss `readBulk` errors to `clean == false`: `simpleredis/simpleredis.go:344-349` (bulk) and `:368-373` (array element).
- Array element whose head is not `$`, `:`, or `+`: `simpleredis/simpleredis.go:378-379` → `errIssue`, `clean == false`.
- `do` maps `err != nil && !clean` to `reusable == false`; I/O errors become `redis:unreachable` via `ioError`; `errIssue` stays `redis:issue?`: `simpleredis/simpleredis.go:299-304`, `ioError` at `:431-436`.
- `release` closes the socket when `reusable` is false and does not append to `idle`: `simpleredis/simpleredis.go:244-248`.
- `startStaticRedis` writes one complete canned RESP per command and never closes mid-payload: `simpleredis/simpleredis_test.go:228-256`.
- In-process `fakeRedis` serves full map-backed replies: `simpleredis/simpleredis_test.go:14-53`.
- Compose already has `redis:7-alpine` and `docker.dragonflydb.io/dragonflydb/dragonfly:v1.40.2`, plus whoami routes `/redis` and `/dragonfly`: `docker-compose.yml:47-75`.
- Pester asserts GET `/redis` and GET `/dragonfly` echo SimpleRedis verbs (including Get/MGet headers): `scripts/integration-tests.Tests.ps1:84-114`.
- Probe uses a per-request key prefix, Set then Get/MGet of that key's own value `"ok"`: `e2e/simpleredisprobe/plugin.go:61-92`.
- Probe Eval script already lists `KEYS[1]` and is Lua 5.1-safe: `e2e/simpleredisprobe/plugin.go:17-23`. Research: `knowledge/research/ext_redis_eval/notes.md`, `knowledge/research/ext_dragonfly_eval/notes.md`.

## Desired
- Unit-test truncated payload: announce `$100\r\n`, write 40 bytes, close. Command error is `redis:unreachable` (not `redis:issue?`). `len(idle) == 0` after the call.
- Unit-test the leak invariant: after that truncated reply, a second command returns the correct value for its own key (poisoned conn must not be reused).
- Unit-test malformed bulk header `$abc\r\n` and an array element head that is neither `$`, `:`, nor `+`: error `redis:issue?`, nothing pooled.
- Coverage of the `io.ReadFull` short-read block (measured `401.52,403.3` on dest) becomes non-zero. Invariant fails if `readBulk` returns `clean == true` on a short read, or if `release` pools a `reusable == false` conn.
- `startStaticRedis` stays as-is; add a fake that can write raw bytes and close mid-stream.
- Tests MUST run against both Redis and Dragonfly (both supported backends).
- Live Get/MGet on both engines still return the caller's own value (no cross-request leak).
- Extend compose + Pester `/redis` `/dragonfly` so that live isolation is asserted on both routes.
- Any Lua added or kept in this change stays Lua 5.1-safe; Dragonfly requires declared `KEYS`.

## Affected
- `simpleredis/simpleredis_test.go` (truncated/malformed fakes and assertions on `idle` / second-command value)
- `docker-compose.yml` (extend Redis and Dragonfly `/redis` `/dragonfly` wiring if live isolation needs more than today's whoami+probe)
- `scripts/integration-tests.Tests.ps1` (extend `/redis` and `/dragonfly`)
- `e2e/simpleredisprobe/plugin.go` (Get/MGet own-value; any script stays KEYS + Lua 5.1)
- `simpleredis/simpleredis.go` only if tests prove `clean`/pool behavior is wrong (finding asks for tests, not a production rewrite)

## Out of scope
- `exec` retry after a lost reply on INCR/INCRBY/EVAL (finding points at test-05; not this ticket).
- perf-01 through perf-08, feat-01, test-01/02/03/06/07 except where this ticket's fake is needed for truncated bytes.
- Changing Redis or Dragonfly image pins unless required to run the live Get/MGet assertions.
- Production `readBulk`/`do`/`release` behavior change when dest already returns `clean == false` on short read.

## Unknowns
- Real Redis and Dragonfly cannot be told to announce `$100` and close after 40 bytes; truncated/`clean == false` is unit-only against a fake unless explore finds a live injection seam.
- How far Pester must go past today's unique-key Get/MGet `"ok"` to prove "caller's own value" under concurrent requests (finding's leak is pool reuse after truncate; conductor also asks live isolation on both engines).
- Coverage block ids `401.52,403.3` are dest-line measurements; they move if `readBulk` is edited.
- Whether compose "extend" is new services/routes or new assertions on the existing `/redis` and `/dragonfly` whoami.

## Tensions
- Finding: truncated payload is a unit fake. Conductor: tests MUST run against Redis and Dragonfly, and live Get/MGet must return the caller's own value. Both apply — unit fake for truncate/`clean == false`; live engines for Get/MGet own-value. Do not treat live engines as a substitute for the truncated-payload unit test.
- Finding names test-05 retry as a secondary concern. That retry policy is out of scope here.
- Conductor Lua 5.1 / Dragonfly KEYS is in Desired even if this ticket adds no new script; dest probe already matches (`e2e/simpleredisprobe/plugin.go:17-23`).
