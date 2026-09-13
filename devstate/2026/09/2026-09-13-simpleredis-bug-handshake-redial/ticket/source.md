# Handshake AUTH/SELECT failure redials a second TCP connection

Handshake AUTH/SELECT failure redials a second TCP connection. Spec std_go_simpleredis_tcp-session “Handshake AUTH or SELECT failure”: one error, MUST NOT open a second TCP connection. Cause: AUTH/SELECT run inside dial(); dial returns do() errors unchanged; exec retries when shouldRetry is true (errUnreachable from EOF, LOADING prefix, exact ERR max number of clients reached). WRONGPASS/NOAUTH already 1 accept. AUTH stall (no reply) is redis:timeout and is not retried. TCP DialContext refuse stays unmarked errUnreachable and MUST still retry.

Agreed how (do not invent another):
- exec must not retry AUTH/SELECT failures.
- Do NOT unmask inside ioError (shared with GET; would break lost-reply retry and Error() redis:unreachable).
- Mark at dial’s return after AUTH/SELECT do(): keep inner Error()/Unwrap() (redis:unreachable, LOADING …, redis:noauth). shouldRetry returns false for that mark BEFORE isUnreachable / isRetryableRedisReply.
- TCP DialContext failure stays unmarked errUnreachable (still retries).
- GET LOADING still retries (do(GET) in exec, never marked).

Out of scope: Eval deadlines, Eval $-1 miss, comments on MSetEX, other simpleredis bugs.

Implement order (include in requirement as desired):
1. FIRST land compiled tests that reproduce (they MUST fail on current dest code). No //go:build bugrepro — they run under default go test -short ./simpleredis/.
2. THEN apply the agreed how.
3. THEN confirm those tests pass, plus existing handshake tests (WRONGPASS / SELECT 99 already 1 conn in pool_test.go; GET LOADING retry in commands_exec_test.go).
4. Do not implement until the failing tests exist in the worktree.

Reuse existing fakes where possible: startFakeRedis, setHandshakeReplies, fake.connections() in simpleredis/fake_redis_test.go and pool_test.go. AUTH close-without-reply needs a listener that accepts, reads AUTH, and closes with no reply. Prefer folding into pool_test.go rather than a tagged file.

Four repros (assert accepts==1 after MaxRetries: 1): AUTH close-no-reply; AUTH OK then SELECT close-no-reply (Pass+Database); AUTH -LOADING Redis is loading the dataset in memory; AUTH -ERR max number of clients reached.
