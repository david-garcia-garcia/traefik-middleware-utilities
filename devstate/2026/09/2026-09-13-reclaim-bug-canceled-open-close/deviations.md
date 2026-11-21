# Deviations

- [x] taken  grace wait stays off the Open stack
  Asked: after canceled bind, call `drop` on this stack (the dest `drop` also waited out grace).
  Instead: `drop` still decrements, sleeps, and zero-grace closes on this stack; the positive-grace timer runs in `waitGraceOrWake`.
  Owner: `reclaim/table.go` `drop`
  Why: dest already waited in an AfterFunc goroutine, not in Open; blocking Open for DefaultGrace would stall Traefik `New`.
  By: implement
  Requester: not asked
