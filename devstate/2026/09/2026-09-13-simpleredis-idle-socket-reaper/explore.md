# Explore
IssueKey: 2026-09-13-simpleredis-idle-socket-reaper

## Concepts

IdleTimeout is a **reuse gate**, not a socket lifetime. `takeIdleConn` (`simpleredis/pool.go`) filters `idleConns` by `now.Sub(conn.lastUsed) < sr.idleTimeout`, then LIFO-pops one survivor. The only caller is `borrow`. Park stamps `lastUsed = time.Now()` in `release`. There is no background closer.

```
  New ──allocates nothing live──► first command dials
                                      │
                                      ▼
                               park idleConns
                                      │
                    no traffic ───────┼────── next borrow
                                      │              │
                                      ▼              ▼
                                 fds stay        sweep ages,
                                 until Close     then reuse or dial
```

Dest already chose this shape. Archived `openspec/changes/archive/2026-09-12-simpleredis-idle-borrow-sweep/` (PR #33) added the whole-list sweep on borrow and wrote `New MUST NOT start a goroutine to close idle sockets` / `MAY keep idle sockets past IdleTimeout until Close` into `openspec/specs/std_go_simpleredis_tcp-session/spec.md`. Usage gotcha `knowledge/devdocs/std_go_simpleredis.md` says the same. Prior ticket `2026-09-12-simpleredis-risk-02-no-idle-reaper` filed `knowledge/debt/2026-09-12-simpleredis-idle-reaper-reclaim.md` rather than start a ticker, because dest product `New` paths do not call `SimpleRedis.Close`.

go-redis (the shape dest copies) also checks `ConnMaxIdleTime` lazily on the next `Get` (`isHealthyConn`), not with a background reaper (`knowledge/research/ext_go-redis_connection-pool/notes.md`). Redis dest compose ships `timeout 0` (never close idle clients) (`knowledge/research/ext_redis_clients_idle-close/notes.md`).

Amplifier (BUG-1): pinned idle sockets are the ones a restart, `CLIENT KILL`, or a positive server `timeout` turns into corpses. Sibling ticket `2026-09-13-simpleredis-stale-pooled-socket-retry` owns borrow validation. This run must not change how `borrow` validates or reports a reused socket.

Yaegi: `simpleredis/resp.go` `watchConnClose` uses `context.AfterFunc` because Yaegi v0.16.1 `interp._select` races when interpreted code selects on a context channel from a goroutine. A ticker reaper that `select`s on stop + ticker from a goroutine would hit that class of bug.

Dest callers that own a SimpleRedis and never Close it:
- `e2e/simpleredisprobe/plugin.go` Traefik `New` — `simpleredis.New` only
- `windowcounter/limiter.go` `Close` — "does not close the injected SimpleRedis"
- `tokenbucket` Redis limiter is built on an injected client the same way

## Decisions

Reproduced the claimed failure. Untracked hunt `TestBugIdleSocketsAreNeverReapedWithoutTraffic` (`//go:build simpleredis_bugs`) from the caller workspace against dest-shaped `simpleredis`:

```
500ms of zero traffic at IdleTimeout=50ms: idle list = 4, server-side open sockets = 4
FAIL
```

That is dest as specified, not a missed closer. `TestStaleIdleHeadIsClosedWhileTailStaysHot` already asserts `New` starts no goroutine and that the next borrow closes a stale head.

Direction comparison (pick at most one):

| Direction | Releases fds during silence? | Product cost |
|---|---|---|
| 1 Background ticker in `New`, stop in `Close` | Yes | Reverses dest spec; every client starts a goroutine; dest production `New` never `Close`s so the reaper **leaks** on the real path; `Close` must race-free the ticker; Yaegi `select` constraint; sibling borrow-path conflict risk if the sweep is hoisted into `borrow` |
| 2 Stamp absolute expiry at park | No | Same comparison as `lastUsed` vs `now` at borrow. Does not address the hunt. Would pretend a lifetime exists that still only runs on borrow. |
| 3 Leave dest; real request-path fix is BUG-1 | No (by design) | Spec and usage already document this. Fd/`maxclients` pressure is bounded by `PoolSize` per plugin instance (default 8). |

Simplicity gate: the only correct fd-release (direction 1) is not small, coherent, or elegant in this tree. Wiring `Close` on Traefik/`windowcounter`/`tokenbucket` is **Out of scope**. Without that wiring, a ticker makes a goroutine leak the production path. Direction 2 does not fix the defect. **This run takes direction 3 and stops after propose.** That stop is success.

Do not rewrite the idle-pool spec or the usage gotcha: they already say the quiet client MAY keep sockets. Propose records the recommendation; no product delta.

## Open questions

- Q: Which of the three fix directions should this run take?
  Rank: additive asked — requirement Desired names pick at most one of three directions, including document dest as specified; this run does not reshape `New` or the idle-pool contract
  Decision: assumed — direction 3. Direction 1 is the only true fd-release and is not small (spec reversal of `New MUST NOT start a goroutine`, dest product `New` paths never `Close`, Yaegi select, Close/reaper race). Direction 2 does not release fds during silence. Simplicity gate: stop after propose with this written recommendation; that stop is success.
  By: explore

- Q: Does production Redis close idle clients (`timeout` > 0), so quiet-time pinning becomes corpses without a restart?
  Rank: additive asked — requirement Unknowns names production Redis timeout is not measured
  Decision: assumed — dest compose/CI is `timeout 0`. Treat the amplifier as restart / `CLIENT KILL` / BUG-1, which does not need a server timeout. Do not bake a server-timeout assumption into a product change this run is not making.
  By: explore

- Q: If direction 3, should this run still rewrite the spec or usage gotcha to "document it"?
  Rank: additive asked — requirement Affected lists spec and usage if the contract changes or is restated
  Decision: assumed — do not rewrite. Spec requirement "Idle connections are pooled" and `knowledge/devdocs/std_go_simpleredis.md` already state `New` starts no reaper and a quiet client MAY keep sockets until `Close`. Restating is noise. Propose writes the recommendation on the card; no product delta.
  By: explore
