---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/util/unsafe.go
title: go-redis internal/util BytesToString
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/util/unsafe.go
---

Build tag `!appengine`.

`BytesToString(b []byte) string` is `unsafe.String(unsafe.SliceData(b), len(b))`.

`StringToBytes(s string) []byte` is `unsafe.Slice(unsafe.StringData(s), len(s))`.

SimpleRedis must not copy this helper (`unsafe` is forbidden on the session source).
