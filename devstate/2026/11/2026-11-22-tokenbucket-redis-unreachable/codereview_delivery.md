# Delivery

## Motivation

El middleware de rate-limit llama a `Redis.Allow` para consumir un token vía Eval. El `rateLimit` de Traefik responde HTTP 500 cuando Redis está caído y no ofrece fail-open ni fail-closed. Este ticket fija ese corte en la ruta de consume de esta biblioteca para que el caller elija la política en vez de heredar ese 500.

Cuando Redis no responde, Traefik responde HTTP 500 `Could not insert/update bucket` sin opción fail-open ni fail-closed (https://github.com/traefik/traefik/issues/12091). Esta biblioteca ya devuelve el error de Eval con un `Status` cero (`Allowed` false) y no elige código HTTP. Nada dedicado prueba que un loopback que rechaza es `errors.Is(err, simpleredis.ErrUnreachable)` con `allowed=false`. El GoDoc de `Allow` no nombra `ErrUnreachable` ni `ErrTimeout`, ni que fail-open o fail-closed lo decide el middleware. El usage todavía pedía comparar errores Redis por el texto de `Error()`, y el contrato de allow describía el corte como la cadena `redis:unreachable`.

Si no se fija, un cambio posterior puede tragar el corte como un deny silencioso, los callers siguen comparando cadenas de display que pueden derivar, y los autores de middleware no pueden ver en esta biblioteca que ellos eligen el código HTTP.

Priority: P3 — spec, docs, tests, or internal clarity — no current user or operator harm

## Implementation

Sin cambio de comportamiento: `Redis.Allow` sigue devolviendo `Status{}, err` si Eval falla. El GoDoc de ese método nombra `simpleredis.ErrUnreachable` (o `ErrTimeout`) con `allowed=false` en un corte de Redis y que fail-open o fail-closed lo decide el middleware, citando el issue de Traefik. Un test unitario autónomo apunta el limiter a `127.0.0.1:1` con `MaxRetries: -1` y afirma `errors.Is(err, simpleredis.ErrUnreachable)` y `allowed=false`. El How to use compara esos sentinels con `errors.Is` / `IsUnreachable`; el texto de display sigue `redis:unreachable`. El delta del contrato de allow nombra el mismo matching y mantiene el corte como error, no como deny sustituto. Los tests de `Error()` existentes siguen como control del texto de display.

## What this changes
**Operators.** None.
**Admin users.** None.
**Developers.** Comparar cortes Redis con `errors.Is` / `IsUnreachable` contra `ErrUnreachable` y `ErrTimeout`; `Allow` documenta eso con `allowed=false`, y fail-open o fail-closed lo decide el middleware.
**End users.** None.
