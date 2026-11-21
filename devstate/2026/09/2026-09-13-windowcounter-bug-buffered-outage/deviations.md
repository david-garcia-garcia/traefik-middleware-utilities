# Deviations

- [x] taken  Peek buffered outage follows Take
  Asked: buffered Take while Redis is down is per-node `limit`, `err=nil` (Desired 3). Unknowns: Peek’s error contract is not stated.
  Instead: buffered Peek returns the same nil-error local observation as Take, without recording a hit.
  Owner: `windowcounter/limiter.go` `peekBuffered`; Language **Peek** on `knowledge/devdocs/std_go_windowcounter.md`
  Why: honouring Take-only would add a second outage contract on the sibling Language already defines as the same observation without a hit.
  By: explore
  Requester: not asked

- [x] taken  buffered Redis-error spec → per-node cap, err=nil
  Asked: dest `std_go_windowcounter_sliding-take` “Redis errors propagate” and `std_go_windowcounter_sync-flush` retained-flush / probe / two-instance require a Redis error on buffered Take/Peek after flush failure or one missed `sync_rate`.
  Instead: buffered outage returns nil error and local deny after this node’s `limit`; exact mode still returns Redis errors; failed flush is stored only to skip Redis contact.
  Owner: `openspec/specs/std_go_windowcounter_sliding-take/spec.md` “Redis errors propagate”; `openspec/specs/std_go_windowcounter_sync-flush/spec.md`
  Why: honouring dest’s fail-closed wording would keep returning a Redis error from buffered Take, which Desired 4 forbids; Desired 6 names this rewrite.
  By: propose
  Requester: confirmed
