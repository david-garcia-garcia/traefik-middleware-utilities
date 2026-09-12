---
url: https://github.com/redis/redis/blob/7.2.4/tests/unit/scripting.tcl
title: EVALSHA NOSCRIPT tests
fetched: 2026-09-11
authority: source
ref: github.com/redis/redis@7.2.4:tests/unit/scripting.tcl
---

EVALSHA of an invalid SHA and of a non-defined SHA both match `{NOSCRIPT*}`.

SCRIPT FLUSH then EVALSHA of a previously loaded digest asserts `{NOSCRIPT*}`. A later EVAL of the same body makes a following EVALSHA succeed.
