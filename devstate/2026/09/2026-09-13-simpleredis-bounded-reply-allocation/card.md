Developer review: in progress — 2026-09-13T17:12:03Z

## What this changes
**Operators.** None.

**Admin users.** None.

**Developers.** None yet versus DestBranch. Explore recorded a chunked `readBulk` and append-on-array-decode shape; product code is unchanged.

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
Explore is written. Product allocation change is not on this branch yet. Propose is next. Simplicity gate: the recorded shape is small enough to implement.

Priority: P2 — real operator pain (Traefik OOM from a handful of header bytes), limited blast until a hostile or buggy peer (or a desynced socket)
Reviewed head: 33ce43f
Owner decision: None.

## Review scores
| Measure | Result | What it means |
| --- | --- | --- |
| Overall readiness | 3/6 | Explore landed; CI on this head is still running; product fix not landed |
| CI proof | 3/6 | build 34770862044 in progress ([Unit](https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34770862044/job/103760084138)) |
| Local tests proof | N/A | Before implement (`localTests: none`) |
| Review resolution | 6/6 | OPEN PR, no reviewer comments |

## Verification
| Check | Result | Evidence |
| --- | --- | --- |
| Branch | 2026-09-13-simpleredis-bounded-reply-allocation pushed | `git` |
| OpenSpec | none | `openspec/` |
| Pull request | https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/86 | pr-host List |
| CI | build 34770862044 in progress https://github.com/david-garcia-garcia/traefik-middleware-utilities/actions/runs/34770862044 | pr-host CI |
| Local tests | none | handoff.yaml localTests |
| PR comments | no comments | no comments.md |

## Specs
None.

## Deviations from the ask
None.

## Follow-up issues
None.

## How this fits together
Local ticket → branch `2026-09-13-simpleredis-bounded-reply-allocation` → stub PR 86 against `master`. Explore recorded the decode shape; propose will write the spec delta.

## Explore Decisions
| Question | Rank | Decision | By |
| --- | --- | --- | --- |
| What first-chunk size keeps dest decode alloc ceilings while stopping header-only 64 MiB `make`? | bounded asked | assumed — `bulkReadChunk = 128 << 10` so 100 KB stays one-shot; a 64 MiB header with no payload allocates 128 KiB then short-reads | explore |
| What array start cap keeps `TestAllocDecodeArray10` while stopping a 1 Mi slot `make`? | bounded asked | assumed — `arrayGrowChunk = 16` so ten elements keep `cap == count`; `make([][]byte, 0, count)` is the same 24 MiB attack | explore |
| Does replacing the accepted-length make+ReadFull SHALL count as stopping at the simplicity gate? | bounded asked | assumed — no; updating the three contract files is the job. A common-path alloc/ns regression at implement is the remaining stop signal | explore |
| Who already owns client identity (address, user, tenant, Host, trust hop) for this change? | additive incidental | assumed — none. The decoder classifies a RESP header; it does not set or rebuild a host fact | explore |

## Before merge
- [ ] Keep SimpleRedis reply memory proportional to bytes the peer actually sent, not the declared in-cap `$` or `*` length
- [ ] Untagged `TotalAlloc` proof for a header-only oversized `$` and `*`
- [ ] Honour the simplicity gate: stop after propose if the correct fix is not small

## Findings
None.

## Axis review
None.

## Agent review details

### Review metrics
| Metric | Value | Why it matters |
| --- | --- | --- |
| Specs in this PR | none | Same list as ## Specs; do not paste diff --stat |
| Open reviewer comments walked | 0 FIX / 0 ANSWER / 0 open | Unanswered review is merge risk |
| Reviewed head | 33ce43f240a0d1978f2e7116393a831667f981d3 | Card must match the branch you measured |

### Stored data model
None.

### Technical review
Best possible solution: not on DestBranch yet. Dest still `make`s from the declared in-cap length before reading payload. Explore's shape is chunked `ReadFull` with first allocation `min(need, 128 KiB)` and array append with start cap 16.

Do we have a high-confidence way to reproduce? Yes. Tagged `TestBugPeerControlledAllocationAmplification` failed on dest: 11 bytes → 67,144,992 allocated; 10 bytes → 25,188,272 (`go test -tags simpleredis_bugs`).

Is this the best way to solve the issue? Yes versus DestBranch: do not `make` the announced size before bytes arrive; keep the common-path one-shot so `$17` and `$102400` benches stay. `io.CopyN` would add a 32 KiB scratch and miss `decodeBulkBytes`.

### Evidence
What I checked:
- Dest `simpleredis/resp.go` `readBulk` / `readReply` `*` case (`origin/master` `a239a9e`, worktree `33ce43f`)
- Tagged reproduction failed as claimed (`TestBugPeerControlledAllocationAmplification`)
- Decode spec SHALL make+ReadFull (`openspec/specs/std_go_simpleredis_resp-decode/spec.md`)
- Alloc ceilings in `simpleredis/bench_test.go` (`decodeBulkBytes = 112`, `decode100KBBytes = 127843`, `decodeArrayBytes = 344`)
- Research: `knowledge/research/ext_redis_resp_bulk-string/`, `ext_redis_proto_max-bulk-len/`, `ext_go-redis_proto_reader-limit/`
- OPEN PR 86, no comments
- CI build 34770862044 in progress on `33ce43f`

### Rank-up moves
None.
