---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/lapi/session.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/lapi/session.go
title: CachePrefix for Redis key namespace
fetched: 2026-09-11
authority: source
---

CachePrefix(cfg): SessionHex(cfg) when CrowdsecMode is stream or alone; else IdentityHex(cfg).
Comment: stream/alone share keys across warn-and-wire; live/none use full identity hex.
JSON fields on session snapshot include RedisCacheEnabled, RedisCacheHost, RedisCacheReadHosts, RedisCachePassword, RedisCacheDatabase, RedisCacheUnreachableBlock.
