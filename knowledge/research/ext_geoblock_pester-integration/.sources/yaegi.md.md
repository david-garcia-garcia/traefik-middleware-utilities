---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md
title: Integration failure postmortem (Yaegi probe)
fetched: 2026-09-11
authority: ticket
ref: david-garcia-garcia/traefik-geoblock@22f09a0:devstate/2026/09/2026-09-08-reclaim-lifecycle/yaegi.md
---

When compose cannot run locally (Windows containers), geoblock used scratch module `D:/repositories/scratch-yaegi-geoblock`:
- `interp.New` with GoPath = plugins-local tree
- `Use(stdlib.Symbols)` + `Use(unsafe.Symbols)` (matches compose useunsafe=true)
- Evaluate real packages like Traefik loader
- Yaegi v0.16.1 (Traefik v3.7.11 pin)

Found: multi-assignment `parsed.RawQuery, parsed.Fragment, parsed.User = "", "", nil` panics interpreted; kills Traefik on update goroutine → all Pester tests fail at API check.

Second finding: optional lifecycle interfaces never match on values from `func() (any, error)` — see debt note.

Recommended CI guard: throwaway module outside plugin tree, depends on pinned yaegi, evaluates plugin GOPATH — do not add yaegi to plugin go.mod.
