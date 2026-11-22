# Issues

- [ ] note large  `knowledge/debt/2026-11-22-tokenbucket-consumer-limiter-scope.md`
  Why: el arreglo de ámbito (un `*Memory` compartido por nombre de middleware, mismo prefijo Redis) pertenece al middleware consumidor, no a esta librería. Origen: https://github.com/traefik/traefik/issues/13706 ; PRs https://github.com/traefik/traefik/pull/13823 y https://github.com/traefik/traefik/pull/13707.
