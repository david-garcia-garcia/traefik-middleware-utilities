# Standards

1. [hard] Leave a trail — `simpleredis/simpleredis.go:245` — `borrow` now has three blocks (wait for a turn, pop idle, dial) and this change deleted the idle-reuse and empty-idle intros; `exec` in the same file still introduces its blocks
   → Restore one-line intros for the wait, idle pop, and dial blocks
   Status: done
   Argument: wait / idle-pop / dial intros restored on borrow.

```
wait := sr.waitLimit()
timer := time.NewTimer(wait)
select {
case <-sr.slots:
...
conn, err := sr.dial()
```

2. [hard] Leave a trail — `simpleredis/simpleredis.go:308` — the new idle-full vs live-cap close decision has no block intro, so the reader must reverse-engineer that `inUse` still includes the socket being released
   → Introduce that block: idle trim only when live is already at cap
   Status: done
   Argument: release now names idle-full vs live-cap close.

```
idleFull := len(sr.idle) >= maxIdleConns
inUse := 0
if sr.slots != nil {
	inUse = sr.liveCap() - len(sr.slots)
}
live := len(sr.idle) + inUse
if sr.closed || (idleFull && live >= sr.liveCap()) {
```

3. [hard] Leave a trail — `e2e/simpleredisprobe/plugin.go:74` — `ServeHTTP`’s comment still lists only the verb-header suite; `?hold=` is a new first block that may 502 and skip those verbs
   → Name the hold-Eval path in the method comment and introduce the block
   Status: done
   Argument: ServeHTTP comment and hold-block intro name ?hold=.

```
// ServeHTTP runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, and Eval, then copies results into headers.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if hold := req.URL.Query().Get("hold"); hold != "" {
		if _, err := m.client.Eval(timeWaitHoldScript, nil, []string{hold}); err != nil {
```

4. [hard] Leave a trail — `simpleredis/live_test.go:54` — new helpers `runLivePoolBackend`, `waitLiveSimpleRedis`, `requireProcNet`, `livePort`, and `countEstablishedToPort` have no job comment; same-package helpers (`startFakeRedis`, `connections`) do
   → Add one succinct job comment on each new helper
   Status: done
   Argument: job comments on runLivePoolBackend, waitLiveSimpleRedis, requireProcNet, livePort, countEstablishedToPort.

```
func runLivePoolBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
```

5. [hard] Bound the ask — `scripts/integration-tests.Tests.ps1:167` — reclaim teardown waits were doubled (30s → 60s); that Describe is not the live-cap work
   → Leave the reclaim timeouts as they were
   Status: done
   Argument: reclaim dispose/close waits restored to 30s.

```
Wait-TraefikPluginLog -Pattern "reclaim_dispose" -TimeoutSeconds 60 | Should -BeTrue
Wait-TraefikPluginLog -Pattern "reclaimprobe_close" -TimeoutSeconds 60 | Should -BeTrue
```
