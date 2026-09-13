# Issues

- [ ] note large  `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`
  Why: recovery can briefly exceed PoolSize when every holder sits in the borrow-to-do or do-to-release gap
- [ ] note large  `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md`
  Why: a recovered panic leaks the fd forever; recovery dials a new socket
