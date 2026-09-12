---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/util/strconv.go
title: go-redis internal/util Atoi
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/util/strconv.go
---

`Atoi(b []byte) (int, error)` is `strconv.Atoi(BytesToString(b))`.

`ParseInt` is `strconv.ParseInt(BytesToString(b), base, bitSize)`.
