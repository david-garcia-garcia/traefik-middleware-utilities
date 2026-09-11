# Standards

1. [hard] Leave a trail — `simpleredis/yaegi_test.go:100` — `RoundTrip` comment says the probe Gets a miss after Del; the body Dels then returns the earlier Get bytes and never Gets again
   ```
   // RoundTrip Inits a client, Sets a key, Gets it, Dels it, then Gets a miss.
   func RoundTrip(host string) string {
   	...
   	if err := client.Del("k"); err != nil {
   		return "del:" + err.Error()
   	}
   	return string(got)
   }
   ```
   → Drop “then Gets a miss”, or Get after Del and return that miss
   Status: done
   Argument: comment now matches Init/Set/Get/Del.
2. [hard] Leave a trail — `simpleredis/simpleredis.go:1` — package comment is “utility routines for interacting” plus a “timetoleave” leftover; `SimpleRedis` comment restates the identifier instead of the pooled TCP RESP job
   ```
   // Package simpleredis implements utility routines for interacting.
   // It supports currently the following operations: GET, MGET, SET, DELETE,
   // and support timetoleave for keys.
   ...
   // A SimpleRedis is used to communicate with redis.
   type SimpleRedis struct {
   ```
   → Say it is a stdlib pooled TCP RESP client (GET/MGET/SET EX/DEL); do not restate the type name
   Status: done
   Argument: package and SimpleRedis comments name the pooled TCP RESP client.
3. [hard] Leave a trail — `simpleredis/simpleredis.go:42` — new type `pooledConn` and unexported methods `close`/`exec`/`borrow`/`release`/`dial`/`do`/`writeCommand`/`readReply`/`readBulk`/`readLine`/`replyError`/`ioError` have no job comment; `borrow` and `dial` are multi-block with no block intros (`ioError` only explains the Yaegi `net.Error` choice)
   ```
   type pooledConn struct {
   	netConn  net.Conn
   	...
   }
   func (c *pooledConn) close() { ... }
   func (sr *SimpleRedis) exec(...) ...
   func (sr *SimpleRedis) borrow() ...
   ```
   → One-line job comment on the type and each func; intro each block in `borrow` (idle vs stale vs dial) and `dial` (TCP vs AUTH vs SELECT)
   Status: done
   Argument: job comments on pooledConn and unexported methods; borrow/dial block intros.
4. [hard] Leave a trail — `simpleredis/simpleredis_test.go:14` — test type `fakeRedis` and helpers `startFakeRedis`/`serve`/`connections`/`bulk`/`readCommand`/`startStaticRedis` have no job comment (`handshakeCounts` is the only helper that has one)
   ```
   type fakeRedis struct {
   	mu      sync.Mutex
   	store   map[string]string
   	...
   }
   func startFakeRedis(...) (*fakeRedis, string) { ... }
   ```
   → Comment the type and each helper with the job (in-process RESP map vs canned-reply listener)
   Status: done
   Argument: job comments on fakeRedis helpers.
