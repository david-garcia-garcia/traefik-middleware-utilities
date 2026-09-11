# Deviations

- [x] taken  omit unused poolSize/poolTimeout from Options
  Asked: finding How to fix lists poolSize/poolTimeout as config fields next to the four timeout knobs; Desired says add the knobs and document that the wait-queue semaphore belongs to perf-01.
  Instead: Options has only DialTimeout, IoTimeout, IdleTimeout, MaxIdleConns. Spec and usage name the live-socket cap and wait queue as perf-01.
  Owner: `simpleredis/simpleredis.go`
  Why: unused fields that look like they bound live sockets would lie until perf-01 lands; this ticket is bound to configurable deadlines, not the semaphore.
  By: explore
  Requester: not asked
