---
url: https://github.com/david-garcia-garcia/traefik-geoblock/blob/22f09a0/knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md
title: Yaegi drops method set on any return
fetched: 2026-09-11
authority: ticket
ref: david-garcia-garcia/traefik-geoblock@22f09a0:knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md
---

Under Yaegi v0.16.1, value from interpreted `func() (any, error)` loses methods:
- Shows as `*struct { Xn int }`, not concrete type
- Interface assertions for sleeper/waker/closer never match
- Concrete pointer assertion `v.(*target)` still works if caller knows type

Probe matrix (scratch yaegi module, plugins-local GoPath):
- Pass concrete into func(any): type switch works; comma-ok panics
- Return through func() (any, error): type switch and comma-ok both fail
- reflect MethodByName: not found

Only Pester integration caught interpreted-only breakage; go test compiles.

Fix direction: explicit hook handoff in create signature (blocked by Yaegi + shared pkg constraint).
