---
url: https://github.com/david-garcia-garcia/traefik-geoblock/pull/83
title: PR #83 reclaim spin-off gap (this repo)
fetched: 2026-09-11
authority: inference
ref: david-garcia-garcia/traefik-geoblock@22f09a0
---

Geoblock PR #83 adds reclaim package and uses it from plugin.go; integration tests exercise full geoblock plugin only.

This spin-off (traefik-middleware-utilities) needs new e2e: fake middleware + Pester + Yaegi load path for reclaim/ alone.

Destination module: github.com/david-garcia-garcia/traefik-middleware-utilities
Destination layout: reclaim/ (not pkg/reclaim/)
