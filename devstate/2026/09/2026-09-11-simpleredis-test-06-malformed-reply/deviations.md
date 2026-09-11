# Deviations

- [x] taken  arity-mismatch tests do not require `idle==0`
  Asked: table-driven canned replies assert the expected error **and** `len(redis.idle) == 0`; cover `Get` / `parseIntegerReply` count-mismatch the same way.
  Instead: `idle==0` only on dirty decoder rows; arity-mismatch rows assert `redis:issue?` and leave the pooled conn.
  Owner: `simpleredis/simpleredis.go` (`Get`, `parseIntegerReply`, `release`)
  Why: those checks run after a successful clean `readReply`; honouring uniform `idle==0` would add a destroy-on-arity branch to verbs whose job is not connection hygiene.
  By: explore
  Requester: not asked
