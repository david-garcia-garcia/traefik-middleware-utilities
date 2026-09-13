# Deviations

- [x] taken  Peek buffered outage follows Take
  Asked: buffered Take while Redis is down is per-node `limit`, `err=nil` (Desired 3). Unknowns: Peek’s error contract is not stated.
  Instead: buffered Peek returns the same nil-error local observation as Take, without recording a hit.
  Owner: `windowcounter/limiter.go` `peekBuffered`; Language **Peek** on `knowledge/devdocs/std_go_windowcounter.md`
  Why: honouring Take-only would add a second outage contract on the sibling Language already defines as the same observation without a hit.
  By: explore
  Requester: not asked
