# Issues

- [ ] note large  `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`
  Why: age cannot distinguish a panic-abandoned checkout from a live socket in the borrow-to-do or do-to-release gap, so turn refill must keep the bare `heldSockets` count
