# Deviations

- [x] taken  path-dispatched Traefik cases instead of one request that sets a header per verb
  Asked: Traefik Pester proves every public command; spec `std_go_simpleredis_resp-commands` says the same request SHALL call Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt and set one response header per verb.
  Instead: `ServeHTTP` maps each public verb to `/<engine>/<verb>`; Pester asserts status and body. Helpers live in `scripts/integration-tests.utils/`; cases live in `scripts/integration-tests.simpleredis.Tests.ps1` (reclaim is a sibling file).
  Owner: `e2e/simpleredisprobe/plugin.go`
  Why: honouring the dump would keep adding headers to one handler whose failures are 502s with no isolated `It`; PathPrefix already matches subpaths, so isolation does not need new routers.
  By: explore
  Requester: confirmed

- [x] taken  HTTP status and body instead of `X-SimpleRedis-*` result headers
  Asked: one result header per verb (or a Hold header on `/hold`).
  Instead: success is HTTP 200 with the Redis payload in the body; command errors are 502 with `err.Error()`; Pester owns the sequences (Set then Get, Eval script in the request, `drop=1`, TIME-wait Eval).
  Owner: `e2e/simpleredisprobe/plugin.go`
  Why: headers existed only because success forwarded to whoami, so the body was whoami HTML; failures already used 502 + body. Mapping SimpleRedis to HTTP lets the tests stay in Pester.
  By: implement
  Requester: confirmed
