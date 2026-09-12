# Deviations

- [x] taken  arity-mismatch tests do not require `idle==0`
  Asked: table-driven canned replies assert the expected error **and** `len(redis.idle) == 0`; cover `Get` / `parseIntegerReply` count-mismatch the same way.
  Instead: `idle==0` only on dirty decoder rows; arity-mismatch rows assert `redis:issue?` and leave the pooled conn.
  Owner: `simpleredis/simpleredis.go` (`Get`, `parseIntegerReply`, `release`)
  Why: those checks run after a successful clean `readReply`; honouring uniform `idle==0` would add a destroy-on-arity branch to verbs whose job is not connection hygiene.
  By: explore
  Requester: not asked

- [x] taken  retry-borrow dirty reply is truncated I/O not `?huh`
  Asked: malformed `?huh` on a reused conn then retry borrow fails (`redis:unreachable`).
  Instead: truncated bulk then close, `MaxRetries: 1`.
  Owner: `simpleredis/commands.go` (`shouldRetry`)
  Why: master retry (#21) retries `redis:unreachable` only, not `redis:issue?`.
  By: merge origin/master
  Requester: not asked
