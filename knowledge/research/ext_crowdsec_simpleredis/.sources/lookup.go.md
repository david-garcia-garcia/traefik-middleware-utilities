---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/decisionscope/lookup.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/decisionscope/lookup.go
title: Decision cache payloads and GetMany
fetched: 2026-09-11
authority: source
---

BannedValue "t", NoBannedValue "f", CaptchaValue "c".
LookupCachedRemediation calls cacheClient.GetMany(LookupCacheKeys(...)); does not import simpleredis.
