---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/openspec/specs/std_go_reclaim_context-lease/spec.md
title: std_go_reclaim_context-lease (Yaegi-relevant clauses)
fetched: 2026-09-11
authority: source
ref: david-garcia-garcia/traefik-geoblock@22f09a0:openspec/specs/std_go_reclaim_context-lease/spec.md
---

Loader-relevant requirements only:

- Table stores `any`; MUST NOT be generic `Table[T]` instantiated from another package (Yaegi panics or fails import).
- `table.go` SHALL import only stdlib.
- `create` SHALL take no arguments — Yaegi cannot call `func(context.Context) (any, error)`.
- Process-wide singleton via `Default` / package `Open`; callers type-assert returned `any`.

(Full grace, holder, and logging requirements are product spec — not loader facts.)
