# Issues

- [ ] note large  `knowledge/debt/2026-11-21-tokenbucket-memory-new-source-scan.md`
  Why: `tokenbucket.Memory` walks every bucket under its mutex on each new source (≈0.8–1.6 ms at 60k–65k sources).
- [ ] note large  `knowledge/debt/2026-11-21-windowcounter-serial-flush.md`
  Why: the buffered flush sends one EVAL per dirty key in series, so the real sync interval grows with key count.
