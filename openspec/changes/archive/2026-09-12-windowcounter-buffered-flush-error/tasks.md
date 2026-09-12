## 1. Killable fake and flush error fields

- [x] 1.1 Track live sockets on `testFakeRedis` and add `Kill` that closes the listener and every accepted connection
- [x] 1.2 Add `lastFlushErr`, `flushFailedAt`, and `lastRedisOK` on `Limiter` under `l.mu`; set `lastRedisOK` on successful seed GET and successful flush
- [x] 1.3 Store `flushPending`'s error in `flushLoop`, `Sleep`, and `Close`; clear `lastFlushErr` on success

## 2. Take and Peek surface

- [x] 2.1 Return `lastFlushErr` from buffered Take and Peek on the existing error slot; still increment `localDelta` on Take and still return local allowed/estimate
- [x] 2.2 When `lastFlushErr` is nil and `now.Sub(lastRedisOK) >= syncRate`, probe with one `flushPending` and return that error
- [x] 2.3 Wrap `parseEvalInt` conversion failures with `fmt.Errorf("%s: %w", simpleredis.RedisIssue, convErr)`
- [x] 2.4 Document exact vs buffered failure next to `syncRate` in `New`, README, and `knowledge/devdocs/std_go_windowcounter.md`

## 3. Proofs

- [x] 3.1 Pending-delta outage: long `syncRate`, one healthy Take, Kill, `SetNowForTest` + one `syncRate`, further Take/Peek error is not nil
- [x] 3.2 Successful flush then kill then Take still returns an error
- [x] 3.3 Exact mode outage still propagates (existing `TestTake_Unreachable` / `TestPeek_Unreachable` stay passing)
- [x] 3.4 Two limiters, two clients, one fake, killed mid-window: nil-error admits combined are at most `limit`
- [x] 3.5 Run `go test -short ./windowcounter/...` until passing

## 4. Validate

- [x] 4.1 Run `openspec validate --change windowcounter-buffered-flush-error --strict`
