---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/plugin.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:plugin.go
title: Traefik Yaegi constructor imports
fetched: 2026-09-11
authority: source
---

Package crowdsec_bouncer_traefik_plugin. Imports appsec, bouncer, configuration, lapi, logger. Does not import pkg/simpleredis or pkg/cache.
New is the Traefik Yaegi constructor; Redis is reached later via lapi.Client → cache.Client.
