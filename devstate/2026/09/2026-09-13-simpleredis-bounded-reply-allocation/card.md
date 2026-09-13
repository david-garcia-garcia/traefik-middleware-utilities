Developer review: in progress — 2026-09-13T17:21:59Z

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
Implement landed. Local tests passed. CI on this head is still queued. Code review is next.

Priority: P2 — real operator pain (Traefik OOM from a handful of header bytes), limited blast until a hostile or buggy peer (or a desynced socket)
Reviewed head: df93dfa
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Fix landed locally; CI on this head is queued |
| CI proof | 3/6 | build 34771368092 queued ([Unit](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34771368092/job/103761454585)) |
| Local tests proof | N/A | Remote PR; CI is the proof axis (`localTests: passed`) |
| Review resolution | 6/6 | OPEN PR, no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-bounded-reply-allocation pushed | `git` |
| OpenSpec | simpleredis-bounded-reply-allocation | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/86 | pr-host List |
| CI | build 34771368092 queued https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34771368092 | pr-host CI |
| Local tests | passed | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
- [std_go_simpleredis_resp-decode](https://github.com/david-garcia-garcia/traefik-middleware-utilities/blob/2026-09-13-simpleredis-bounded-reply-allocation/openspec/changes/simpleredis-bounded-reply-allocation/proposal.md) — modified

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket → branch `2026-09-13-simpleredis-bounded-reply-allocation` → stub PR 86 against `master`. Implement grew bulk/array buffers from arrived bytes; seven-axis review is next.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What first-chunk size keeps dest decode alloc ceilings while stopping header-only 64 MiB `make`? | bounded asked | assumed — `bulkReadChunk = 128 << 10` so 100 KB stays one-shot; a 64 MiB header with no payload allocates 128 KiB then short-reads | explore |
| What array start cap keeps `TestAllocDecodeArray10` while stopping a 1 Mi slot `make`? | bounded asked | assumed — `arrayGrowChunk = 16` so ten elements keep `cap == count`; `make([][]byte, 0, count)` is the same 24 MiB attack | explore |
| Does replacing the accepted-length make+ReadFull SHALL count as stopping at the simplicity gate? | bounded asked | assumed — no; updating the three contract files is the job. A common-path alloc/ns regression at implement is the remaining stop signal | explore |
| Who already owns client identity (address, user, tenant, Host, trust hop) for this change? | additive incidental | assumed — none. The decoder classifies a RESP header; it does not set or rebuild a host fact | explore |

## Before merge
- [x] Keep SimpleRedis reply memory proportional to bytes the peer actually sent, not the declared in-cap `$` or `*` length
- [x] Untagged `TotalAlloc` proof for a header-only oversized `$` and `*`
- [x] Honour the simplicity gate: the recorded shape is small enough to implement

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | 0 added / 1 modified | Same list as ## Specs |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | df93dfa03f8255e53961dbae2697e1bcb8257484 | Card must match the branch you measured |

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
- OPEN PR 86, no comments
- CI build 34771368092 queued on `df93dfa`

### Rank-up moves
None.
