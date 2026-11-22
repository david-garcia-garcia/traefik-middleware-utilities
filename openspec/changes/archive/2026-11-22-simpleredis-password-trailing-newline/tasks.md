## 1. Config.Pass GoDoc

- [x] 1.1 Extend the GoDoc on `Config.Pass` in `simpleredis/config.go`: AUTH receives the field byte for byte, with no trimming; a trailing newline from a Secret or file makes AUTH fail; cite https://github.com/traefik/traefik/issues/12078. Do not change `New`, `authArgs`, `applyDefaults`, `User`, or AUTH behavior. Verify the comment names byte-for-byte send, no trimming, and the upstream URL, and `git diff` of non-test session files is only that GoDoc.

## 2. Live trailing-newline AUTH test (traefik/traefik#12078)

- [x] 2.1 Copy as-is `TestTraefik12078_PasswordWithTrailingNewline` from the read-only reference `simpleredis/traefik_issue12078_e2e_test.go` into `simpleredis/traefik_issue12078_e2e_test.go` in this worktree. Self-contained file. Reuse existing `runForEachLiveEngine` (`SIMPLEREDIS_LIVE_REDIS_AUTH`, `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`) and `newTestRedis`. Do not create shared helper files. Do not copy other issues' tests. Any helper added in this file must be suffixed `12078`. Cases stay: `control_exact_password` (`Pass` `"secret"`, want AUTH) and `secret_with_trailing_newline` (`Pass` `"secret\n"`, want `redis:noauth`). Verify the function and both subtests exist and the file compiles under `go test -short -count=1 ./simpleredis/` (live cases skip).

- [x] 2.2 Run `TestTraefik12078_PasswordWithTrailingNewline` against the requirepass engines (`SIMPLEREDIS_LIVE_REDIS_AUTH=127.0.0.1:16381`, `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH=127.0.0.1:16382`, password `"secret"`). Verify exact `"secret"` authenticates (Get success or `redis:miss`) and `"secret\n"` fails with `redis:noauth` on each set engine.

## 3. Package gate

- [x] 3.1 Local gate before push: `go vet ./...` ; `go test -short -count=1 ./...` ; live tests of `./simpleredis/` with `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH` (and the dest live addrs already used by this package). Verify all three commands exit 0.
