---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/lapi/client.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/lapi/client.go
title: LAPI client constructs and closes cache
fetched: 2026-09-11
authority: source
---

Prepare: cfg.RedisCachePassword, _ = configuration.GetVariable(cfg, "RedisCachePassword").
Client.New: cacheClient = &cache.Client{}; cacheClient.New(log, RedisCacheEnabled, RedisCacheHost, RedisCacheReadHosts, RedisCachePassword, RedisCacheDatabase, CachePrefix(config)).
Close: if cacheClient != nil { cacheClient.Close() }. Sleep keeps cache. Cache() returns cacheClient.
hydrateRangeMembership: cacheClient.Get(decisionscope.RangeIndexKey).
Does not import pkg/simpleredis; Redis is reached only through cache.Client.
