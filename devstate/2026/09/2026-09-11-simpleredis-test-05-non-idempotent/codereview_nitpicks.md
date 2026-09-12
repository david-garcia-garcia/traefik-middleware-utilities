# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis.go:248` — `retryBackoff` holds the shifted/jittered/capped wait in local `d`; the body uses it only as the backoff duration being computed
   → Rename to a role name such as `backoff` or `jitteredBackoff` through the whole function
   Status: done
   Argument: renamed `d` to `backoff` in `retryBackoff`.

2. [hard] Symmetry and consistency — `e2e/respdroprelay/main.go:155` — In `readRawRESP`, the bulk-string branch builds the wire reply in `out` while the array branch uses `raw` for the same accumulating-bytes role
   → Use one identifier (e.g. `raw`) in both `$` and `*` branches
   Status: done
   Argument: `$` branch now accumulates in `raw`, same as `*`.
