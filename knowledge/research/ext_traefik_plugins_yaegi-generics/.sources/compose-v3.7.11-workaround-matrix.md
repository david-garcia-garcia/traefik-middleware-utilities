---
ref: david-garcia-garcia/traefik-geoblock@22f09a0:knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/compose-v3.7.11-workaround-matrix.md
title: Yaegi generic workaround matrix (Traefik v3.7.11)
fetched: 2026-09-11
authority: source
---

Non-generic `map[string]any` + type-assert in caller: PASS.
Same-package `Table[T]` with package-level var: PASS.
Cross-package `*reclaim.Table[*BIN]` at package scope: panic nodeType2.
Cross-package type alias or embedding of `Table[*T]`: panic nodeType2.
Recommended shared-library shape: non-generic table of `any` + caller type-assert.
