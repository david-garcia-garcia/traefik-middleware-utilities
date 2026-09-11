# Dragonfly BLPOP is fully supported

Dragonfly accepts Redis `BLPOP` and documents the same empty-list block. Live I/O-timeout proof can use one unique empty key on Dragonfly the same way as on Redis. ([dragonflydb.io compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-blpop.md](.sources/compatibility-blpop.md); [dragonflydb.io BLPOP](https://www.dragonflydb.io/docs/command-reference/lists/blpop), [.sources/blpop.md](.sources/blpop.md))

## Empty list blocks that connection

Syntax `BLPOP key [key ...] timeout`. If none of the keys exist, Dragonfly blocks **that connection** until a push or until `timeout` seconds (double; `0` waits forever). Expired timeout returns a nil multi-bulk. ([dragonflydb.io BLPOP — Blocking behavior](https://www.dragonflydb.io/docs/command-reference/lists/blpop), [.sources/blpop.md](.sources/blpop.md))

Compatibility matrix: List / `BLPOP` = **Fully supported**. Verification pin on that page is Dragonfly v1.40.0 (compose in this repo is `v1.40.2`). "Fully supported" is command-surface, not byte-for-byte Redis. ([compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-blpop.md](.sources/compatibility-blpop.md))

## Unblock order can differ from Redis

When several keys are listed, Dragonfly may return an element from any key that became non-empty after a `MULTI`/`EXEC` or script (parallel execution). Irrelevant for a **one-key** stall on an empty list. ([dragonflydb.io BLPOP](https://www.dragonflydb.io/docs/command-reference/lists/blpop), [.sources/blpop.md](.sources/blpop.md))

Declare the list key if the stall is ever wrapped in `EVAL` (Dragonfly undeclared-key rule). The planned proof is a top-level `BLPOP`, not Eval. (`ext_dragonfly_eval`)

## DEBUG SLEEP is not the stall

Dragonfly's DEBUG page exists and calls DEBUG an internal testing command. It does not document a `SLEEP` subcommand. The compatibility table has no Redis `DEBUG` row (`SCRIPT DEBUG` is Unsupported; JSON.DEBUG is a different command). Do not use `DEBUG SLEEP` on Dragonfly. ([dragonflydb.io DEBUG](https://www.dragonflydb.io/docs/command-reference/server-management/debug), [.sources/debug.md](.sources/debug.md); [compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-blpop.md](.sources/compatibility-blpop.md))

`CLIENT PAUSE` is Fully supported on Dragonfly — same shared-CI reason to avoid it as Redis. ([compatibility](https://www.dragonflydb.io/docs/command-reference/compatibility), [.sources/compatibility-blpop.md](.sources/compatibility-blpop.md))
