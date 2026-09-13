# Test coverage

Ticket job (requirement Desired / proposal Why): in-memory per-key Gate — Allow admits a real backend attempt, Report records the boolean on a saturating credit bucket, trip to OPEN, then exponential cooldown.

1. [hard] Assertion does not prove the job — `backendbackoff/allow.go:103` — idle OPEN key is deleted and treated as unseen; `TestAllow_IdleKeyPresumedHealthy` (`backendbackoff/gate_test.go:107`) advances 2m (past both TTL and the 1s cooldown) and only asserts `allowed`. Reverting the expire delete still admits: OPEN past `openUntil` becomes a HALF-OPEN probe
   → After idle Allow, assert credit is `B` and one failure does not re-OPEN (or idle with TTL elapsed while cooldown has not)
   Status: done
   Argument: TestAllow_IdleKeyPresumedHealthy now uses TTL 3s / cooldown 10s and asserts credit B plus one failure still admits.
2. [hard] Assertion does not prove the job — `backendbackoff/allow.go:71` — successful probe sets `credit = budget`; `TestReport_ProbeSuccessRetainsN` (`backendbackoff/gate_test.go:202`) only asserts later `retryAfter == 2s`. Reverting the restore still trips on the first of `trip()`'s B failures and leaves n=1 cooldown
   → Assert credit is `B` (and CLOSED) after the successful probe Report
   Status: done
   Argument: TestReport_ProbeSuccessRetainsN asserts credit B and CLOSED.
3. [hard] Edge case untested — `backendbackoff/allow.go:34` — HALF-OPEN Allow after `probeUntil` with an outstanding probe takes the slot; `TestAllow_ProbeAfterCooldown` (`backendbackoff/gate_test.go:162`) only denies the second Allow during the lease. `(none)` for elapsed lease
   → Advance past the probe lease with no Report and assert the next Allow is admitted
   Status: done
   Argument: TestAllow_LostProbeLeaseAdmits.
4. [hard] Critical path untested — `backendbackoff/allow.go:63` — Report on OPEN returns nil and must not change credit/`n`; `(none)`
   → After a trip, Report success while OPEN and assert the next Allow is still denied with the same cooldown
   Status: done
   Argument: TestReport_OpenIgnored.
5. [hard] Edge case untested — `backendbackoff/allow.go:83` — success credit is capped at `B`; `TestReport_SuccessCredits` (`backendbackoff/gate_test.go:88`) starts from `B-1` so the sum stays below `B`
   → Report success at full credit (or until credit would exceed `B`) and assert credit stays `B`
   Status: done
   Argument: TestReport_SuccessCapsAtB.
6. [judgement] Happy path only — `backendbackoff/gate.go:112` — `New` rejects TripFailures < 1, BaseCooldown ≤ 0, MaxCooldown < Base, Jitter not in [0, 1), TTL < 1s; only FailureRatio is tested (`TestNew_RejectsInvalidRatio`)
   → Assert each remaining constructor error, or skip as non-admission arms
   Status: skipped
   Argument: judgement; non-admission constructor arms.
7. [judgement] Happy path only — `backendbackoff/allow.go:109` — map cap calls `dropOne` at 65536; `(none)`
   → Assert a 65537th distinct key does not grow the map past cap, or skip (cost)
   Status: skipped
   Argument: judgement; 65536-key test cost.
8. [judgement] Happy path only — `backendbackoff/gate.go:163` — `cooldownDuration` jitter-on and MaxCooldown cap; tests freeze `Jitter: 0` and only use n=0/1 below the cap
   → Assert jittered wait when Jitter > 0 and a high-n cooldown equals MaxCooldown, or skip as non-deny arms
   Status: skipped
   Argument: judgement; tests freeze Jitter 0; cap is non-deny math.
