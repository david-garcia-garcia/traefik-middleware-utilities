# Standards

1. [hard] Leave a trail — `simpleredis/chaos_pool_test.go:64` — `serve`, `honestReply`, `openSockets`, `accepts`, and `chaosGetContext` have no job comment; dest `fakeRedis.serve` / `openSockets` / `connections` do (`// serve answers AUTH/SELECT/…`, `// openSockets is how many accepted sockets are still open`)
   ```
   func (f *chaosFake) serve(conn net.Conn) {
   ```
   → Add a one-line job comment on each, matching dest fake helpers
   Status: done
   Argument: job comments on serve, honestReply, openSockets, accepts, chaosGetContext.
2. [hard] Leave a trail — `simpleredis/yaegi_errorpath_test.go:103` — `writeGopathErrorpath`, `evalErrorpath`, and `TestYaegiErrorpath_*` have no job comment; dest `writeGopathClientprobe` / `evalClientprobe` / `TestYaegi_*` do (`// evalClientprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).`)
   ```
   func writeGopathErrorpath(t *testing.T, goPath string) {
   	t.Helper()
   	writeGopathFile(t, goPath, "errorpathprobe", "errorpath.go", errorpathprobeSrc)
   }
   ```
   → Add a one-line job comment on each, matching dest Yaegi helpers
   Status: done
   Argument: job comments on writeGopathErrorpath, evalErrorpath, and each TestYaegiErrorpath_*.
