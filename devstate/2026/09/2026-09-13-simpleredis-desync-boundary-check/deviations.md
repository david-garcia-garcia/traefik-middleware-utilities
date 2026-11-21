# Deviations

- [x] taken  pre-write leftover refuse inside `do` in addition to the asked post-read gate
  Asked: after a successful parse, leftover bytes return the value and `reusable = false`.
  Instead: also refuse the next write on that socket when `Buffered() != 0`, returning `redis:issue?` so AUTH leftover cannot be parsed as SELECT without editing `dial`.
  Owner: `simpleredis/resp.go`
  Why: `dial` ignores `reusable`; honouring post-read only would leave SELECT reading AUTH leftover. The check stays inside `do`.
  By: implement
  Requester: not asked
