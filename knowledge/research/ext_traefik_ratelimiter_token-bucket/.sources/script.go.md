---
url: https://github.com/redis/go-redis/blob/v9.21.0/script.go
title: go-redis Script.Run EVALSHA then EVAL
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis/v9@v9.21.0:script.go
---

Traefik pins `github.com/redis/go-redis/v9 v9.21.0` and calls `script.Run`.
`NewScript` SHA-1s the source. `Run`: `EvalSha` first; if `NOSCRIPT` then `Eval`.
This product's v1 path that uses only EVAL should not copy `Run`/EVALSHA.
