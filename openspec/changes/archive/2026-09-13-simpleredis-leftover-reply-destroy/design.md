## Context

Dest `do` returns `reusable = true` after any clean `readReply`. `release` refreshes `lastUsed` on reuse, so a warm desynced socket never hits `IdleTimeout`. Explore reproduced `Get(k5) = "STRAY"` on dest. Package constraints: stdlib only, no `unsafe`/cgo/generics, keep Yaegi workarounds (`context.AfterFunc`, no `errors.As` on a package-local struct, no `net.Error` assert). Two other agents own `pool.go` and `commands_exec.go`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Gate reuse on an empty reader after a successful parse, inside `do`.
- Protect AUTH then SELECT on the same socket without editing `dial`.
- Land untagged default-suite proofs for own-key, discard+redial, and no churn.

**Non-Goals:**
- Draining leftover bytes.
- Sending `HELLO` or speaking RESP3.
- Editing `simpleredis/BUGS.md`, `pool.go`, or `commands_exec.go`.
- Local `go test -race` (no C toolchain).

## Decisions

1. **Post-read `Buffered() == 0` is the reuse gate.** After `readReply` returns a well-formed value, `reusable` is true only when the reader has no leftover. Leftover: return those slots and `reusable = false`. Alternative: drain until empty — rejected; the count of extra values is unknown.

2. **Pre-write refuse stays in `do`.** If `Buffered() != 0` before `writeCommand`, do not write; return `reusable = false` and `redis:issue?` so `dial` closes on AUTH leftover without reading it as SELECT. Alternative: honor `reusable` in `dial` — rejected; other agents own `pool.go`. Compliant `+OK` already leaves the buffer empty (`TestAuthAndSelectOncePerDial`).

3. **Stray fake lives next to the other fakes.** Port `startStrayExtraReplyFake` from `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` into `fake_redis_test.go`. Wire is `$5\r\nSTRAY\r\n`, not the prose `STRAYYY`. Tests in `resp_test.go`. Untagged. Alternative: keep `//go:build bugrepro` — rejected; Desired names the default suite.

4. **No-churn proof is a connection count.** Keep `TestConnectionIsReused` (25 Gets → 1 accept). Do not treat pass/fail without that count as enough.

5. **Yaegi only if the existing probe fits.** `bufio.Reader.Buffered()` is stdlib. `TestYaegi_NewGetSetDel` already covers empty-buffer reuse interpreted. Add an interpreted Get against the compiled stray fake only by extending `clientprobeSrc`; do not add a second interpreter harness.

6. **LOADING/TRYAGAIN stay reusable.** Those `-` replies are complete and leave `Buffered() == 0`. Do not treat leftover as those retries.

## Risks / Trade-offs

- [False-positive churn on a compliant peer] → Mitigation: `Buffered() == 0` after one complete value; `TestConnectionIsReused` asserts 1 accept.
- [Handshake AUTH leftover parsed as SELECT] → Mitigation: pre-write refuse in `do` (decision 2).
- [Parallel edits to `pool.go` / `commands_exec.go`] → Mitigation: keep the hunk inside `do`.
- [Yaegi `Buffered()` surprise] → Mitigation: stdlib method; optional interpreted Get if `clientprobe` extends cheaply.

## Migration Plan

Session source plus unit tests. Rollback is revert. No stored data change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
