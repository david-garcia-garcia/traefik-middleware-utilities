# Spec

1. [missing] `openspec/changes/add-leakybucket/specs/std_go_leakybucket_sync-flush/spec.md` — Requirement: Live Redis and Dragonfly prove both engines — Yaegi live SHALL run the same Take scenarios interpreted
   `leakybucket/yaegi_test.go:61` wires `exactPourThenLeak` to `IdleThenTake` (fill, drain, Take allowed). Compiled `leakybucket/live_test.go:89` asserts deny at capacity before the drain Take. Interpreted live never runs that deny step (only fake-TCP `PourToCapThenDeny`).
   Status: done
   Argument: Yaegi `IdleThenTake` now denies at capacity before drain; `c8855c3`.
