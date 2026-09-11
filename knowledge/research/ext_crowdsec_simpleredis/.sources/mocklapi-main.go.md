---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/tests/e2e/mock/mocklapi/main.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:tests/e2e/mock/mocklapi/main.go
title: e2e Redis mock speaking SimpleRedis RESP
fetched: 2026-09-11
authority: source
---

Comment: Redis mock (RESP arrays as spoken by pkg/simpleredis, plus inline GET).
serveRedis: TCP; GET/MGET verdicts or miss; SET, DEL, AUTH, SELECT +OK.
Not a unit test of pkg/simpleredis; stand-in for plugin redis cache path (primary miss, replica hardcoded 1.2.3.4=f / 1.2.3.5=t).
