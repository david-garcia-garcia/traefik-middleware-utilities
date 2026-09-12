# Deviations

- [x] taken  capability cache on groupWriteMu instead of the ask's existing mu
  Asked: cache native vs Lua under the existing mutex that guarded the idle list.
  Instead: a dedicated `groupWriteMu` on `SimpleRedis`.
  Owner: `simpleredis/simpleredis.go`
  Why: DestBranch replaced generic `mu` with `idleConnsMu` for unused sockets only; sharing that lock would mix two jobs.
  By: implement
  Requester: not asked
