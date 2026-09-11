# Fix reclaim lifecycle under Yaegi

Take (do not note) the debt at `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md`.

Traefik v3.7.11 pins Yaegi v0.16.1. A value returned through an interpreted `func() (any, error)` comes back as a synthesized struct with no methods. Optional `Sleep` / `Wake` / `Close` on a reclaim stored value never match, so the four-event lifecycle is inert in production plugins. There is no point in having lifecycle hooks that do not work with Yaegi.

Fix `Open`'s signature: explicit hooks instead of optional interface discovery, so Sleep/Wake/Close match under Yaegi.

Tests: introduce tests that import the actual Yaegi interpreter for leaner coverage of the Yaegi behavior. Still use e2e with Pester for a final wrap-up of key parts. As part of test coverage, implement a test host (which probably only writes to logs in each of the events) for the reclaim table that uses all the lifecycle hooks.

Destination branch is master.
