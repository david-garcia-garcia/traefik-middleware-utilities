# Fix peer-controlled RESP reply allocation in simpleredis

Fix ONE production defect in the `simpleredis` package of the Go repo at `D:\repositories\traefik-middleware-utilities` (module `github.com/david-garcia-garcia/traefik-middleware-utilities`).

## THE SIMPLICITY GATE — this overrides the workflow's "Done when"

Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, **do not implement it**. Stop after propose, write up the options with their costs, and hand the decision back to the human. Stopping with a clear written recommendation is a SUCCESS outcome, not a failure.

## The defect

The RESP decoder sizes its allocation from a peer-supplied length **before reading a single payload byte**, so a handful of header bytes costs tens of megabytes.

Measured:

- `$67108864\r\n` — 11 wire bytes caused 67,133,952 bytes allocated. Amplification ~6,103,087x.
- `*1048576\r\n` — 10 wire bytes caused 25,182,640 bytes allocated. Amplification ~2,518,264x.

Root cause, `readBulk` in `simpleredis/resp.go` around line 209:

```go
	data := make([]byte, length+2)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
```

and the array branch of `readReply` around line 155:

```go
		values := make([][]byte, count)
		for i := 0; i < count; i++ {
```

The existing caps (`maxBulkLength` 64 MiB, `maxArrayCount` 1 Mi entries) bound one allocation but not the aggregate: `PoolSize` concurrent commands multiply it, so the default pool of 8 reaches roughly 512 MiB from 88 bytes of input. In a Traefik plugin that allocation lands in the router process, and an OOM-killed Traefik is an outage for every route, not just the rate-limited ones.

This needs a hostile or buggy peer, so it ranks below the availability bugs. It is worth fixing anyway because of the amplification factor, and because a desynced socket (a separate known defect) can make the decoder read attacker-influenced *cache values* as length headers.

## Reproduction (read these; they are untracked in the main checkout, so read them by absolute path)

- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-4
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`

Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugPeerControlledAllocation' -v`

## Fix direction (evaluate, do not follow blindly)

Do not trust the declared length as an allocation instruction. The two smallest shapes:

- For bulk payloads, read incrementally into a buffer that grows as bytes actually arrive, capped at `maxBulkLength`. A chunked `io.CopyN` loop into a `bytes.Buffer`, or a grow-as-you-go `append`, keeps the memory proportional to what the peer really sent.
- For arrays, append elements as they decode instead of sizing the slice from the count.

Keep the existing caps as the hard ceiling. Watch the allocation cost on the normal path: small replies are the overwhelming majority and must not get slower or allocate more than they do today. If incremental reading measurably regresses the common case, that is a signal to reconsider the shape.

## Constraints

- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (this runs as a Traefik plugin): no reflection, no `unsafe`, no new non-stdlib imports.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code. Match it exactly.
- There is an existing fuzz suite (`simpleredis/resp_fuzz_test.go`, `FuzzReadReply` and `FuzzParseLen`) and a large `simpleredis/resp_test.go`. Both must stay green. The decoder's existing contract about dirty streams matters: a reply it cannot frame must still mark the socket unusable, and `redis:issue?` / `redis:unsupported-reply` must not turn into `redis:unreachable` or vice versa. Do not change which sentinel each malformed shape produces.
- Preserve existing invariants: in-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks.
- Five sibling agents are fixing other `simpleredis` bugs in parallel. BUG-3 (per-read deadline) also edits `readBulk`, so a conflict there is likely. Keep your diff surgical and confined to allocation — do not also change deadline handling.

## Regression test

Add a permanent test to the **default** (untagged) suite that fails before your fix and passes after: assert that an oversized `$` header and an oversized `*` header, each followed by no payload, allocate an amount proportional to what actually arrived rather than to the declared length. Measure with `runtime.ReadMemStats` `TotalAlloc` around the call, as the existing reproduction does. Prefix any new fake or helper with something bug-specific so it cannot collide with sibling branches.

## Verify and report (later phases)

Run `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`, the fuzz seed corpus, and `go test -bench . ./simpleredis/` to show the normal path did not regress. Re-run the tagged reproduction to show it now passes.
