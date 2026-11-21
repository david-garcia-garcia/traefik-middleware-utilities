# Deviations

- [x] taken  GET previous only when this instance counted that key (`expireAt` set), not on every in-memory `localDelta == 0`
  Asked: on buffered Take, if the previous key is in memory and `localDelta == 0`, GET and set `redisKnown`.
  Instead: GET previous when it is in memory, `localDelta == 0`, and `expireAt > 0` (this instance counted it as current). First-sight previous stays a one-time GET seed.
  Owner: `windowcounter/limiter.go`
  Why: GET whenever `localDelta == 0` would GET previous on every buffered Take after first sight, which dest already forbids (`MUST NOT GET Redis on every buffered call merely because a delta is pending`).
  By: implement
  Requester: not asked
