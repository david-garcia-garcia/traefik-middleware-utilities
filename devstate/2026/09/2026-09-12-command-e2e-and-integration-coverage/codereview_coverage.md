# Test coverage

1. [hard] Assertion does not prove the job — `scripts/integration-tests.simpleredis.Tests.ps1:58` — Path-dispatched Traefik proofs must show Expire/ExpireAt actually ran; `It "POST …/expire succeeds"` and `It "POST …/expireat succeeds"` only assert HTTP 200 on the verb response (`$expire.StatusCode | Should -Be 200`) after Set, while production is `e2e/simpleredisprobe/plugin.go:264-294` (`serveExpire` / `serveExpireAt` call `client.Expire` / `client.ExpireAt` then `writeOK(nil)`). Reverting those handlers to `writeOK` without dialing SimpleRedis would leave both Its green; the old monolithic probe at least tied `X-SimpleRedis-Expire` / `X-SimpleRedis-ExpireAt` to the same block as the client calls.
   → After Expire/ExpireAt, Eval TTL on the same key (mirror `It "POST …/msetex lands a positive TTL"`) or assert TTL changed vs the post-Set baseline.
   Status: done
   Argument: Expire 30 then TTL ≤ 30; ExpireAt +90 then TTL > 60.

2. [hard] Assertion does not prove the job — `simpleredis/yaegi_test.go:292` — Ticket expands Yaegi live to the full public verb set; new `LiveVerbs` Expire/ExpireAt arms only return on non-nil error (`if err := client.Expire(...); err != nil { return "expire:" + ... }` and the same for `ExpireAt`) with no TTL or stored-value check, while compiled live already proves outcome in `simpleredis/commands_e2e_test.go:104-122` (`expireThenTTL` / `expireAtThenTTL` + `assertLiveTTLPositive`). A no-op `Expire`/`ExpireAt` in the client would still yield `"ok"` from `evalClientprobe(… LiveVerbs …)`.
   → After each Expire/ExpireAt in `LiveVerbs`, Eval TTL (or Get) must fail if the command did not land.
   Status: done
   Argument: LiveVerbs Evals TTL after Expire/ExpireAt and fails unless remaining seconds > 60.
