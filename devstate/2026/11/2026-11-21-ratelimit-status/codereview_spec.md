# Spec

1. [judgement] Design drift — `openspec/changes/ratelimit-status/design.md:65` — D3 nombra `durationFromMicros` usado por `Period`, `Reset`, `Delay` y `RetryAfter`; el código tiene `durationFromNanos` (solo `refillPeriod`) y la misma regla en línea en `newStatus`; D2 calcula `Period` como `burst / limitPerMicro` y el código como `burst × 1s / rate`; D5 y `tasks.md` 3.1 nombran `newStatus(sliding, limit, window, current, previous)` y el código no tiene `window`
   Fix: Alinear D2, D3, D5 y las tareas 2.1 y 3.1 con el código entregado (el comportamiento es el mismo)
   Status: done
   Argument: `design.md` D2 (`Period`/`Reset` como `burst × 1s / rate`, `refillPeriod`), D3 (`durationFromNanos`, regla en línea en `newStatus`) y D5 (`newStatus(sliding, limit, current, previous)`), y `tasks.md` 2.1 y 3.1, alineados con el código. Sin cambio de comportamiento.
