# Deviations

- [x] taken  sweep on next borrow instead of quiet-time close with no traffic
  Asked: after pooling n sockets, stopping traffic, and waiting past IdleTimeout, the fake’s accept-minus-close count is 0 with no later borrow.
  Instead: `takeIdleConn` sweeps the whole idle list on the next borrow, closes stale outside the lock, and reuses the newest survivor. A fully quiet client keeps its sockets until `Close` or a later command.
  Owner: `simpleredis/pool.go`
  Why: honouring zero sockets with no borrow would add a `New` ticker that only `Close` stops, and dest product callers do not call `SimpleRedis.Close`; the ticket’s preferred option is the sweep.
  By: explore
  Requester: not asked
