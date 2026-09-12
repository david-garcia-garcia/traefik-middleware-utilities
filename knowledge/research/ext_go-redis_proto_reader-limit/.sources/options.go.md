---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/options.go
title: go-redis Options.ReadBufferSize
fetched: 2026-09-12
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:options.go
---

`ReadBufferSize` is the size of the `bufio.Reader` buffer for each connection. Godoc: larger buffers help large responses; smaller buffers help large pools. Default comment: 32KiB (32768 bytes).

`initOptions`: `ReadBufferSize == 0` → `proto.DefaultBufferSize`. RESP3 clamps below `proto.MinRESP3ReadBufferSize` (128) up to that minimum so a push header fits. Still an I/O buffer, not a maximum RESP bulk or array length.
