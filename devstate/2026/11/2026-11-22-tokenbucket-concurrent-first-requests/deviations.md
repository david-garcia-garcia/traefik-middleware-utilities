# Deviations

- [x] taken  cliente vivo con PoolSize 16 en el archivo del issue, no `waitLiveClient`
  Asked: misma puerta que `TestLive_Limiter` vía `runForEachLiveEngine` / `waitLiveClient`, y PoolSize 16 para que las 16 peticiones se solapen.
  Instead: `runForEachLiveEngine` para skip y tabla Redis/Dragonfly; `simpleredis.New` en `traefik_issue14328_e2e_test.go` con PoolSize 16, MaxIdleConns 16, PoolTimeout 2s, más espera local `waitLiveClient14328`.
  Owner: `tokenbucket/limiter_e2e_test.go` (`waitLiveClient`)
  Why: honrar las dos letras a la vez añade un parámetro PoolSize a `waitLiveClient`, un helper cuyo trabajo es esperar con Config por defecto (PoolSize 8). Out of scope prohíbe helpers compartidos; el repro 14328 ya construye el cliente con PoolSize 16.
  By: explore
  Requester: not asked
