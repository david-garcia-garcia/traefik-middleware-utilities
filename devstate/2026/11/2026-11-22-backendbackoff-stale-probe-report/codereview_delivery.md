# Delivery

## Motivation

When a key trips OPEN, the gate waits out the cooldown and then admits one HALF-OPEN probe. That probe's Report is the only outcome that may close the key, reopen it, or change n. Cooldown math is already specified to use only that probe result.

Allow records only a boolean probeOutstanding. Report takes the key and a success flag, with no identity of the attempt. In HALF-OPEN the first Report to arrive is treated as the probe, even when it belongs to a request admitted before the trip.

With TripFailures 5, Jitter 0, and BaseCooldown 1s: six requests are admitted while CLOSED; five fail fast and trip OPEN while the sixth is still in flight; the cooldown elapses and probe P is admitted; the sixth request then Reports success and the gate closes with full credit; the real probe's failure only subtracts one credit. The key is CLOSED, credit 4, n 0, although the probe failed. The same steal happens when a probe lease expires: a replacement probe is admitted and the late Report of the first one takes its slot.

Left alone, concurrency reopens an unhealthy backend (flapping onto it) or keeps a healthy one OPEN with n inflated toward MaxCooldown.

Priority: P2 — real operator and end-user pain from a stale report deciding the probe, with limited blast radius

## Implementation

Allow returns a Permit that carries the opaque key and that entry's generation; a deny is the zero Permit, whose Allowed() is false. Report takes that Permit instead of a key. Each entry holds a generation from a gate-wide counter, assigned when the slot is created and bumped on every state transition (trip, OPEN to HALF-OPEN, a replacement probe after an expired lease, HALF-OPEN close or reopen). CLOSED admits do not bump, so every in-flight attempt of the current CLOSED era still reports. Report applies only when the permit's generation matches the entry; a mismatch, a zero permit, or a permit from an expired then recreated key is a no-op (no credit, state, n, or TTL change). There are no legacy Allow/Report signatures.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** `Allow` returns a `Permit`; `Report` takes that permit instead of the key; `Allowed()` replaces the bool; a permit from another generation is ignored.
**End users.** A HALF-OPEN outcome is no longer stolen by an in-flight request from before the probe, so an unhealthy backend is not reopened by a stale success and a healthy one is not held OPEN by a stale failure.
