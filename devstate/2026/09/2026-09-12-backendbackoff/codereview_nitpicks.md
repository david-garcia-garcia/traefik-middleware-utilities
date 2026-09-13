# Nitpicks

1. [hard] Name for the scope — `backendbackoff/allow.go:132` — `dropExpired` ranges the map as `source`; this body only deletes that map key, and Allow/Report/loadEntry already name the same role `key`
   ```
   func (g *Gate) dropExpired(now time.Time) {
   	for source, entry := range g.keys {
   		if !now.Before(entry.expireAt) {
   			delete(g.keys, source)
   		}
   	}
   }
   ```
   → `for key, entry := range g.keys` (same role as Allow)
   Status: done
   Argument: dropExpired ranges key.
2. [hard] Name for the scope — `backendbackoff/allow.go:140` — `dropOne` deletes one slot as `source`; sibling methods in this file use `key`
   ```
   func (g *Gate) dropOne(now time.Time) {
   	g.dropExpired(now)
   	if len(g.keys) < maxMemorySources {
   		return
   	}
   	for source := range g.keys {
   		delete(g.keys, source)
   		return
   	}
   }
   ```
   → `for key := range g.keys`
   Status: done
   Argument: dropOne ranges key.
3. [hard] Name for the scope — `backendbackoff/gate.go:166` — `u` is a letter placeholder for the [0, 1) draw used in the jitter formula
   ```
   func (g *Gate) cooldownDuration(n int) time.Duration {
   	wait := g.cfg.BaseCooldown
   	...
   	u := g.rng.Float64()
   	jittered := time.Duration(float64(wait) * (1 + g.cfg.Jitter*(2*u-1)))
   	...
   	return jittered
   }
   ```
   → `unitInterval := g.rng.Float64()` (or `jitterSample`); keep the formula
   Status: done
   Argument: jitterSample.
