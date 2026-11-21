# Test coverage

1. [hard] Edge case untested — `windowcounter/limiter.go:28` — `if redis.call("PTTL", KEYS[1]) < 0 then EXPIRE` skip arm (key already has TTL; MUST NOT refresh) has no test that would fail if EXPIRE ran on every hit. `TestTake_ExpireOnFirstHit` asserts first-hit `EXPIRE … 20` only. `TestRepro_ExactExpireNotRetriedAfterFailure` asserts a leftover no-TTL key sends EXPIRE on the second Take. `TestTake_NThenDeny` Takes again after TTL is set but asserts admit/deny only. `(none)` asserts expire command count stays put or TTL is unchanged.
   → Assert a second successful Take on a key that already has TTL does not send EXPIRE (or that TTL is unchanged)
   Status: done
   Argument: `TestTake_ExpireOnFirstHit` second Take asserts expire count unchanged (`4da382f`).
