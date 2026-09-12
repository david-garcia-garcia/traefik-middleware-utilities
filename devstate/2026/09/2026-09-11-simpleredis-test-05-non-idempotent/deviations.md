# Deviations

- [x] taken  warm Get before drop Incr and again before drop Eval
  Asked: warm the drop client with a pass-through Get, then Incr and Eval through the drop client.
  Instead: warm Get, Incr, warm Get, then Eval.
  Owner: `e2e/simpleredisprobe/plugin.go`
  Why: the drop-relay drops INCR/EVAL only after a session already forwarded a command; after lost-reply Incr the socket is gone, so Eval needs its own warm Get or the relay would pass the first EVAL through.
  By: implement
  Requester: not asked

- [x] taken  do not retry redis:timeout
  Asked: match go-redis command retry, which retries I/O timeouts.
  Instead: never retry `redis:timeout`.
  Owner: `simpleredis/simpleredis.go`
  Why: this client's `ioTimeout` is 1s; retrying timeouts would stall a Traefik request for several seconds.
  By: implement
  Requester: not asked

- [x] taken  retry every verb like go-redis, including INCR/EVAL
  Asked: retry only idempotent verbs; INCR/INCRBY/EVAL return `redis:unreachable` without a second send.
  Instead: every verb uses go-redis-shaped retry; INCR/INCRBY/EVAL double-apply after a lost reply is accepted.
  Owner: `simpleredis/simpleredis.go`
  Why: go-redis already retries every command; a per-verb `retryDeadPool` gate is a variation the copied client does not have.
  By: implement
  Requester: confirmed
