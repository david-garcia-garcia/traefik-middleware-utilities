# Nitpicks

1. [hard] Clear conditions — `simpleredis/chaos_pool_test.go:100` — `default` wraps the honest reply; the reader must know `chaosDelay`..`chaosTruncated` and that `rand.Intn(5)` also yields 0 to see who enters
   ```
   switch rand.Intn(5) {
   case chaosDelay:
   	// sleep, then honestReply
   case chaosCloseNoReply:
   	return
   case chaosLoading:
   	// LOADING
   case chaosTruncated:
   	// short bulk, return
   default:
   	if _, err := io.WriteString(conn, f.honestReply(args)); err != nil {
   		return
   	}
   }
   ```
   → `chaosHonest = 0`; `case chaosHonest:` (same write as today's default)
   Status: done
   Argument: chaosHonest = 0 and case chaosHonest.
