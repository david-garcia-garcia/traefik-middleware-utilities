# Yaegi drops the method set of a value returned as `any`

IssueKey: 2026-09-11-reclaim-table
Size: large
Action: note

## Why this follow-up

Traefik v3.7.11 pins Yaegi v0.16.1. A value returned through an interpreted `func() (any, error)` comes back as a synthesized struct with no methods. Optional `Sleep` / `Wake` / `Close` on a reclaim stored value never match, so the four-event lifecycle is inert in production plugins.

## Why it was not taken

Fixing it means changing `Open`'s signature (explicit hooks instead of optional interface discovery). This ticket ports the geoblock reclaim API so consumers can switch imports. An API fork here would not be a small take.

## Risks

Compiled `go test` stays green while Traefik never sleeps or closes stored values. Consumers may believe Sleep releases idle cost when it does not under Yaegi.

## Context

Source measurement: `david-garcia-garcia/traefik-geoblock` @ `22f09a0` `knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md`. Local copy of the Yaegi facts: `knowledge/research/ext_traefik_plugins_yaegi-generics/`.
