Developer review: ready for review — 2026-09-13T17:28:17Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** `readBulk` grows a `$` payload in 128 KiB `ReadFull` chunks instead of `make(length+2)` up front. The `*` decoder appends from a start cap of 16 instead of `make(count)`. `TestAllocAmpInCapHeaderDoesNotAllocateAnnouncedSize` locks both header-only cases.

**End users.** None.

## Motivation
On DestBranch, SimpleRedis already rejects a `$` or `*` length above the package caps (`64 MiB` bulk, `1 Mi` array slots) as `redis:issue?` before `make`. A header *at* those caps is still treated as an allocation size. Eleven wire bytes `$67108864` plus CRLF allocate about 64 MiB; ten wire bytes `*1048576` allocate about 24 MiB of slice headers. The default pool of 8 concurrent commands multiplies that to about 512 MiB from 88 header bytes.

That allocation lives in the Traefik process. An OOM-killed router is an outage for every route, not only the rate-limited ones. The peer must be hostile or buggy (or a desynced socket feeding cache bytes as length headers), so this is below the availability bugs, but the amplification is millions to one.

```mermaid
sequenceDiagram
    participant Peer
    participant Decoder
    participant Traefik
    Peer->>Decoder: in-cap 64 MiB dollar header, no payload
    Decoder->>Decoder: make 64 MiB before any payload byte
    Peer->>Decoder: close or stall
    Note over Traefik: router process holds tens of MiB per command
```

## Merge readiness
Ready for review. All required CI checks succeeded. Local tests passed. Checklist is empty.

Priority: P2 — real operator pain (Traefik OOM from a handful of header bytes), limited blast until a hostile or buggy peer (or a desynced socket)
Reviewed head: b6931e2
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 6/6 | CI green; no open comments; local tests passed |
| CI proof | 6/6 | build 34771609776 succeeded ([Unit](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34771609776/job/103762103357)) |
| Local tests proof | N/A | Remote PR; CI is the proof axis (`localTests: passed`) |
| Review resolution | 6/6 | OPEN PR, no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-bounded-reply-allocation pushed | `git` |
| OpenSpec | simpleredis-bounded-reply-allocation (archived) | `openspec/changes/archive/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/86 | pr-host List |
| CI | build 34771609776 success https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34771609776 | pr-host CI |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_resp-decode](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/openspec/changes/archive/2026-09-13-simpleredis-bounded-reply-allocation/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket → branch `2026-09-13-simpleredis-bounded-reply-allocation` → PR 86 against `master`. Decode now grows from arrived bytes; CI on `b6931e2` succeeded.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What first-chunk size keeps dest decode alloc ceilings while stopping header-only 64 MiB `make`? | bounded asked | assumed — `bulkReadChunk = 128 << 10` so 100 KB stays one-shot; a 64 MiB header with no payload allocates 128 KiB then short-reads | explore |
| What array start cap keeps `TestAllocDecodeArray10` while stopping a 1 Mi slot `make`? | bounded asked | assumed — `arrayGrowChunk = 16` so ten elements keep `cap == count`; `make([][]byte, 0, count)` is the same 24 MiB attack | explore |
| Does replacing the accepted-length make+ReadFull SHALL count as stopping at the simplicity gate? | bounded asked | assumed — no; updating the three contract files is the job. A common-path alloc/ns regression at implement is the remaining stop signal | explore |
| Who already owns client identity (address, user, tenant, Host, trust hop) for this change? | additive incidental | assumed — none. The decoder classifies a RESP header; it does not set or rebuild a host fact | explore |

## Before merge
None.

## Findings
None.

## Axis review
[Standards](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_standards.md) — 0 total, 0 pending, 0 completed
[Nitpicks](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_nitpicks.md) — 0 total, 0 pending, 0 completed
[Spec](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_spec.md) — 0 total, 0 pending, 0 completed
[Security](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_security.md) — 0 total, 0 pending, 0 completed
[Performance](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_performance.md) — 0 total, 0 pending, 0 completed
[Dead](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_dead.md) — 0 total, 0 pending, 0 completed
[Test coverage](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/devstate/2026/09/2026-09-13-simpleredis-bounded-reply-allocation/codereview_coverage.md) — 0 total, 0 pending, 0 completed

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | b6931e2c19be8ad4bd294d08696fd6d9d75b4dd7 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: chunked `ReadFull` with first allocation `min(need, 128 KiB)` and array append with start cap 16. Dest `make`s the announced size before any payload byte.

Do we have a high-confidence way to reproduce? Yes. Tagged `TestBugPeerControlledAllocationAmplification` failed on dest (11 bytes → 67,144,992; 10 bytes → 25,188,272) and passed after (148,928 and 17,760; still `redis:unreachable`).

Is this the best way to solve the issue? Yes versus DestBranch. Common-path decode benches stayed 2 allocs / 48 B (`$17`) and 2 allocs / 106520 B (`$102400`). `io.CopyN` would have missed those ceilings.

### Evidence
What I checked:
- `go vet ./simpleredis/` ok
- `go test ./simpleredis/ -count=1` passed
- `go test ./... -count=1 -short` passed
- Fuzz seeds `FuzzReadReply` / `FuzzParseLen` passed
- Alloc ceilings unchanged vs dest (`TestAllocDecodeBulk` 2/48, `TestAllocDecodeArray10` 11/272, `TestAllocDecodeBulk100KB` 2/106521)
- Tagged reproduction passed after the fix
- Seven-axis review: all none
- OPEN PR 86, no comments
- CI build 34771609776 succeeded (Lint, Unit, Unit race, Integration Tests, Integration Tests Redis, Integration Tests Dragonfly, Go E2E Redis, Go E2E Dragonfly)

### Rank-up moves
None.
