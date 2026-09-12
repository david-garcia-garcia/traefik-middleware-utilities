---
url: https://github.com/dragonflydb/dragonfly/issues/59
title: Early Dragonfly EVALSHA SHA mismatch
fetched: 2026-09-11
authority: vendor
---

Historic report: some Lua bodies hashed to a different SHA1 on early Dragonfly than on Redis; Redigo client-side SHA matched Redis and EVALSHA then returned NOSCRIPT.

Not the v1.40.2 pin. Live SCRIPT EXISTS of the Go digest is the regression check. Do not compute a second Dragonfly-only hash.
