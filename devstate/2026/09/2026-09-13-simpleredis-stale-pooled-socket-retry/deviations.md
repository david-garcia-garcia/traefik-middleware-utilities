# Deviations

- [x] taken  skip-idle on remaining attempts instead of a free extra send
  Asked: an I/O failure on a reused socket does not consume MaxRetries, including when MaxRetries is -1.
  Instead: remaining attempts of that command skip idle and still consume MaxRetries as dest.
  Owner: `simpleredis/commands_exec.go`
  Why: a dead unused socket on this platform can fail after write succeeds, which is the same shape as a lost reply; a free extra send would retry Incr when MaxRetries is off.
  By: implement
  Requester: not asked
