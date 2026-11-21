# Security

1. [judgement] Unbounded work under lock (availability) — `simpleredis/pool.go:171` — abandon scans the unbounded turnWaiters queue linearly while it holds inUseMu, up to twice per waiter, so a saturated pool degrades into an O(waiters²) lock convoy on the borrow/release path
   Fix: Make abandon O(1) (mark the waiter abandoned under inUseMu and let freeInUseTurn skip abandoned heads, or keep an intrusive list), and add a test that holds thousands of queued waiters and bounds the time inUseMu is held
   Status: skipped
   Argument: Judgement: queue depth is bounded by callers past PoolSize within PoolTimeout (200 ms default), each waiter leaves by PoolTimeout or ctx, and the grant shift is a native memmove; O(1) tombstone removal adds state to a path measured on purpose. No backlog was measured; not applied unattended.
   Note:
   ```
   Source: request rate to any route that borrows (client-controlled), plus a slow or stalled Redis that keeps PoolSize turns held.
   Sink: inUseMu, which every uncontended take (pool.go:154) and every release (freeInUseTurn, pool.go:90) also need.
   Both the PoolTimeout time.AfterFunc and context.AfterFunc(ctx) run abandon, so a waiter granted or removed by the first still pays a second full scan.
   freeInUseTurn also shifts the whole queue with copy on every grant (pool.go:93).
   The old channel semaphore removed waiters in O(1) in the runtime. Under Yaegi the interpreted range loop costs more per element.
   Not measured: this is a lock-convoy amplification risk under saturation, not a proven DoS. The spec delta names no bound on queued waiters.
   ```
