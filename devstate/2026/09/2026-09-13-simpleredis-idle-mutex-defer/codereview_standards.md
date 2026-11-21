# Standards

1. [hard] Leave a trail — `simpleredis/pool.go:178` — the keep-or-close block still says `Close only when…` after this change moved `conn.close()` out of `parkIdleConn`; that method now returns false and the caller closes
   → Rewrite the block intro to the verdict this body actually returns (do not park when shut, or when idle is already `maxIdleConns` and live is at `liveCap()`)
   Status: done
   Argument: rewrote the block intro to the verdict `parkIdleConn` returns (`f328c39` product plus follow-up).
