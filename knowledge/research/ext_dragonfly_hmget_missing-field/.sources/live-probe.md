---
url: redis://127.0.0.1:16380
title: Live Dragonfly EVAL of HMGET on a missing hash
fetched: 2026-11-22
authority: source
ref: local dragonfly EVAL tb14297:missing
---

EVAL of `redis.call('hmget', KEYS[1], 'last', 'tokens')` on key `tb14297:missing` returned four bulk strings: type `boolean`, value `false`, type `boolean`, value `false`.
