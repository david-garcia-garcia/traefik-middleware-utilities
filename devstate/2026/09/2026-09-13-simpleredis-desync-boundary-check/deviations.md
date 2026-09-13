# Deviations

- [x] taken  pre-write leftover refuse inside `do` in addition to the asked post-read gate
  Asked: after a successful parse, leftover bytes return the value and `reusable = false`.
  Instead: also refuse the next write on that socket when `Buffered() != 0`, returning `redis:unreachable` so AUTH leftover cannot be parsed as SELECT without editing `dial`.
  Owner: `simpleredis/resp.go`
  Why: `dial` ignores `reusable`; honouring post-read only would leave SELECT reading AUTH leftover. The check stays inside `do`.
  By: implement
  Requester: not asked

- [x] taken  pre-write leftover sentinel is `redis:unreachable`, not `redis:issue?`
  Asked: refuse the next write when leftover is already in the reader (first taken row).
  Instead: that refusal uses `errUnreachable`, the same text as a short bulk read, not a new sentinel and not `redis:issue?`.
  Owner: `simpleredis/resp.go`
  Why: `redis:issue?` is not retried; a poisoned socket is recovered by a fresh dial. Handshake leftover still does not retry because `dial` marks `handshakeFailed`.
  By: implement
  Requester: approved
