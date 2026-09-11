---
url: https://github.com/dragonflydb/dragonfly/blob/main/tests/dragonfly/eval_test.py
ref: dragonflydb/dragonfly@main:tests/dragonfly/eval_test.py
title: eval_test.py — proactor_threads in tests
fetched: 2026-09-11
authority: source
---

@dfly_multi_test_args examples pass proactor_threads: 4 with default_lua_flags for eval integration tests.

Pattern for CI: proactor_threads can be set explicitly when running dragonfly in tests; not part of official minimal compose.
