# Deviations

- [x] taken  path-dispatched Traefik cases instead of one request that sets a header per verb
  Asked: Traefik Pester proves every public command; spec `std_go_simpleredis_resp-commands` says the same request SHALL call Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt and set one response header per verb.
  Instead: `ServeHTTP` switches on the path case (`/redis/get`, `/dragonfly/msetexat`, …); Pester asserts one case per `It`.
  Owner: `e2e/simpleredisprobe/plugin.go`
  Why: honouring the dump would keep adding headers to one handler whose failures are 502s with no isolated `It`; PathPrefix already matches subpaths, so isolation does not need new routers.
  By: explore
  Requester: confirmed
