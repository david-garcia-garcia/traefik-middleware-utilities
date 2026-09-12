## 1. Sentinel and borrow

- [ ] 1.1 Add unexported `errNotFromNew` beside `errPoolWait` in `simpleredis/simpleredis.go` (`errors.New(RedisUnreachable)`). Comment: client that did not come from New
- [ ] 1.2 Return `errNotFromNew` from `borrow` when `inUseTurns` is nil. Do not call `ensureInUseTurns`. Leave closed-client on `errUnreachable`

## 2. Tests

- [ ] 2.1 Assert `shouldRetry(errNotFromNew)` is false next to `TestShouldRetryPoolWaitIsFalse`
- [ ] 2.2 Add a zero-value `Get` test: `Error()` is `redis:unreachable`, elapsed `< 8ms`, `cap(sr.inUseTurns) == 0`
- [ ] 2.3 Smoke every exported command on `&SimpleRedis{}` (non-empty `MGet` / `MSetEX` / `MSetEXAt`) plus `Close` twice with no panic; commands that reach the session return `redis:unreachable`
- [ ] 2.4 Run `go test -short ./simpleredis/...` until those tests pass

## 3. Usage and validate

- [ ] 3.1 Add a gotcha on `knowledge/devdocs/std_go_simpleredis.md`: a client not from `New` fails the first command immediately with `redis:unreachable` and is not retried
- [ ] 3.2 Run `openspec validate --change simpleredis-zero-value-no-retry --strict`
