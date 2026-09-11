---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/cache/cache_test.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/cache/cache_test.go
title: cache tests that construct SimpleRedis
fetched: 2026-09-11
authority: source
---

Import of pkg/simpleredis.
Test_nextReader / Test_NewKeepsRedisReadersByPointer: allocate *SimpleRedis pointers; New(..., true, "127.0.0.1:1", two read hosts, prefix "p") must keep distinct writer/reader pointers (crowdsec-bouncer-traefik-plugin#381).
Test_ClientCloseRedis: New Redis mode then Close twice and nil Close.
Test_GetManyUnreachable: Redis New against 127.0.0.1:1 → cache:unreachable.
No protocol fake in this file; Redis-mode tests hit a closed/refused port or un-Inited zero clients.
