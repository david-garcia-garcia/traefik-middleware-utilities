# Delivery

## Motivation

`Gate.Allow` comprobaba `context.Context` solo para devolver `ctx.Err()` antes de tomar el mutex. Eso acopla la máquina de estados síncrona (crédito, OPEN/HALF-OPEN/CLOSED) al ciclo de vida del request HTTP, duplicando una decisión que el middleware ya puede tomar antes de intentar admisión. Mientras el contrato vivo y los devdocs exigían pasar `req.Context()` y negar en cancelación, cualquier caller que quisiera una admisión puramente local debía arrastrar contexto sin beneficio para la lógica interna.

Priority: P3 — contrato público y documentación; sin daño operativo inmediato en este árbol (solo el paquete `backendbackoff` importa la API aquí).

## Implementation

Se cambió la firma a `Allow(key string)`, se eliminó el early return por contexto y se actualizaron tests, probe Yaegi, benchmark y devdocs. El delta OpenSpec en `openspec/changes/backendbackoff-allow-nocontext/` sustituye el escenario de cancelación interna por responsabilidad del caller antes de llamar; `Report` sigue sin contexto. La spec del catálogo vivo se fusionará en archive.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** BREAKING: invocar `Allow(key)` sin contexto; comprobar cancelación o deadline del request antes de llamar; no esperar `context.Canceled` ni `DeadlineExceeded` desde `Allow`.
**End users.** None.
