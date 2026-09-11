---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/cache/cache.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/cache/cache.go
title: cache package Redis client wiring
fetched: 2026-09-11
authority: source
---

Import: github.com/maxlerebourg/crowdsec-bouncer-traefik-plugin/pkg/simpleredis
redisCache: writer *simpleredis.SimpleRedis, readers []*simpleredis.SimpleRedis, prefix, atomic counter.
nextReader: writer if no readers; else round-robin readers.
get: nextReader.Get(prefixed key); RedisMiss→cache:miss; RedisUnreachable→cache:unreachable; empty value also miss.
getMany: nextReader.MGet; omit empty/nil slots; RedisUnreachable→cache:unreachable.
set: writer.Set(prefixed, []byte(value), duration); log on error.
delete: writer.Del(prefixed); log on error.
close: writer.Close then each reader.Close.
Client.New isRedis: allocate &SimpleRedis{} per write host and each read host, Init(host, pass, database); comment: hold by pointer after Init so pool mutex is not copied.
prefixed: prefix+":"+key, or key when prefix empty.
Memory path uses ttl_map, not SimpleRedis.
