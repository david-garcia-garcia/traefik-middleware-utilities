---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/configuration/configuration.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/configuration/configuration.go
title: Redis cache config fields and defaults
fetched: 2026-09-11
authority: source
---

Config JSON: redisCacheEnabled, redisCacheHost, redisCacheReadHosts, redisCachePassword, redisCachePasswordFile, redisCacheDatabase, redisCacheUnreachableBlock.
New() defaults: RedisCacheEnabled false; RedisCacheHost "redis:6379"; RedisCacheReadHosts []; password and database ""; RedisCacheUnreachableBlock true.
