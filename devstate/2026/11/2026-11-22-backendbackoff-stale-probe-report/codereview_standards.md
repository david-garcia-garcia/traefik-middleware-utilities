# Standards

1. [hard] Leave a trail — `backendbackoff/allow.go:60` — generation mismatch returns nil with no block intro
   Fix: Comment that a mismatched generation is a no-op without TTL refresh
   Status: done
   Argument: comment before the gen mismatch return in Report.
   Quote:
      ```
      if entry.gen != permit.gen {
      	return nil
      }
      // Expire is Allow's job. An admitted attempt that Reports after TTL still lands.
      ```
2. [hard] Consume before produce — `backendbackoff/zzz_probe_permit_test.go:217` — `newGateProbePermit` and `tripProbePermit` copy `newTestGate` and `trip` in the same package
   Fix: Delete both helpers and call `newTestGate` and `trip`
   Status: done
   Argument: probe permit tests call newTestGate and trip; copies deleted.
   Quote:
      ```
      func newGateProbePermit(t *testing.T) *Gate {
      	t.Helper()
      	gate, err := New(Config{
      		FailureRatio: 0.30,
      		TripFailures: 5,
      		BaseCooldown: time.Second,
      		MaxCooldown:  10 * time.Second,
      		Jitter:       0,
      		TTL:          time.Minute,
      	})
      ```
3. [hard] Leave a trail — `knowledge/devdocs/std_go_backendbackoff.md:65` — Gotcha still says reporting a deny poisons credit after Report ignores a zero permit
   Fix: Drop the poison-credit sentence; denied and zero permits are ignored
   Status: done
   Argument: Gotcha now says denied and zero permits are ignored.
   Quote:
      ```
      - Report only real attempts the gate admitted. Reporting denies poisons credit and can strand OPEN.
      ```
