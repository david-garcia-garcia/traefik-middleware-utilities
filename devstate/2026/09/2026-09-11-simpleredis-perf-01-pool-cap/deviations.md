# Deviations

- [x] taken  `New(Config)` instead of `Init(host, pass, database)`
  Asked: const-only pool; Init stays three strings; no public pool knobs.
  Instead: `simpleredis.New(Config)` copies knobs at construction; pool, timeout, and retry knobs freeze there and are readable on the client. There is no `Init` method and no already-initialized error.
  Owner: `simpleredis/simpleredis.go`
  Why: knobs that must not change after construction need a copy-in surface; hardcoded caps cannot be set by callers without Config; construction is `New`, not a method on a zero value.
  By: implement
  Requester: confirmed
