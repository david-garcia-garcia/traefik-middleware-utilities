# Nitpicks

1. [hard] Name for the scope — `simpleredis/live_test.go:105` — `killPooledIdleAddrForTest` names ADDR only; the body still KILLs by CLIENT ID when ADDR returns 0
   ```
   func killPooledIdleAddrForTest(t *testing.T, sr *SimpleRedis) {
   	t.Helper()
   	sr.mu.Lock()
   	if len(sr.idle) == 0 {
   		sr.mu.Unlock()
   		t.Fatal("no idle pooled conn to kill")
   	}
   	pooled := sr.idle[0]
   	addr := pooled.netConn.LocalAddr().String()
   	sr.mu.Unlock()

   	killed := clientKillFromSidecarForTest(t, sr.host, "ADDR", addr)
   	if killed == 0 {
   		id := clientIDOnConnForTest(t, sr, pooled)
   		killed = clientKillFromSidecarForTest(t, sr.host, "ID", id)
   	}
   	if killed == 0 {
   		t.Fatalf("CLIENT KILL killed 0 clients (ADDR %s)", addr)
   	}
   }
   ```
   → `killPooledIdleAddrOrIDForTest` (or `killPooledIdleForTest`); keep ADDR then ID
   Status: done
   Argument: Renamed to killPooledIdleAddrOrIDForTest; ADDR then ID unchanged.
2. [hard] Symmetry and consistency — `simpleredis/simpleredis_test.go:308` — `peerCloseFake.acceptsCount` vs sibling `fakeRedis.connections` for the same TCP-accept count; the new test already says "opened N connections"
   ```
   func (f *fakeRedis) connections() int {
   	f.mu.Lock()
   	defer f.mu.Unlock()
   	return f.conns
   }
   func (f *peerCloseFake) acceptsCount() int {
   	f.mu.Lock()
   	defer f.mu.Unlock()
   	return f.accepts
   }
   ```
   → Rename `acceptsCount` to `connections`
   Status: done
   Argument: peerCloseFake.connections matches fakeRedis.connections.
