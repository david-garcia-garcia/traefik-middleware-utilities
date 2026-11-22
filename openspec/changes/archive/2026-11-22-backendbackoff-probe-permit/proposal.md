## Why

`Allow` records only a boolean `probeOutstanding` and `Report(key, success)` carries no identity of the attempt. In HALF-OPEN the first `Report` to arrive is taken as the probe result, even when it comes from a request admitted before the trip or from a probe whose lease already expired. Reproduced: 6 admits while CLOSED, 5 fast failures trip the key, the probe is admitted, the slow 6th request reports success and closes the gate with full credit, then the real probe's failure only subtracts one credit (CLOSED, credit 4, `n` 0). Under concurrency this reopens unhealthy backends (flapping) and keeps healthy ones blocked with an inflated `n`.

## What Changes

- **BREAKING** `Allow(key string) (Permit, time.Duration, error)`. `Permit` is a concrete struct in `backendbackoff` with unexported fields (key, generation). `Permit.Allowed() bool` replaces the old bool. A deny returns the zero `Permit`.
- **BREAKING** `Report(p Permit, success bool) error`. The key comes from the permit, so a report cannot name a different key than the attempt it describes.
- Each entry carries a generation taken from a gate-wide counter. It is set when the entry is created and bumped on every state transition (CLOSED→OPEN, OPEN→HALF-OPEN, a replacement probe after an expired lease, HALF-OPEN→CLOSED, HALF-OPEN→OPEN). An admitted `Allow` stamps the current generation on its permit.
- `Report` applies only when the permit's generation equals the entry's current generation. Any other report (older era, expired probe lease, denied/zero permit, missing or recreated entry) is a no-op: no credit change, no state change, no `n` change, no TTL refresh.
- No parallel legacy methods. All in-tree callers (tests, benches, Yaegi probes, README snippet, usage packet) migrate in this change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_backendbackoff_allow`: Allow returns a `Permit`; Report takes that permit and ignores permits from another generation.
- `std_go_backendbackoff_cooldown`: only the report of the active probe's permit decides the HALF-OPEN outcome and `n`.

## Impact

- `backendbackoff/gate.go` (`memEntry` generation field, gate counter, `Permit` type), `backendbackoff/allow.go` (`Allow`, `Report`, transitions).
- Every in-tree caller of `Gate.Allow` / `Gate.Report` (32 Allow and 18 Report call sites, all in `backendbackoff/zzz_*` plus the Yaegi probe source), `README.md` snippet, `knowledge/devdocs/std_go_backendbackoff.md`.
- Downstream importers outside this repo must migrate: the public signature changes. Sibling tickets `backendbackoff-expiry-list` (adds list pointers to `memEntry`) and `backendbackoff-jitter-below-cap` (`cooldownDuration`) touch the same package; keep hunks minimal.
