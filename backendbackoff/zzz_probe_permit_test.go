package backendbackoff

import (
	"testing"
	"time"
)

func TestProbePermit_StaleSuccessDoesNotDecideProbe(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	inflight := make([]Permit, 6)
	for i := 0; i < 6; i++ {
		permit, _, err := gate.Allow("k")
		if err != nil || !permit.Allowed() {
			t.Fatalf("closed admit %d: allowed=%v err=%v", i, permit.Allowed(), err)
		}
		inflight[i] = permit
	}
	for i := 0; i < 5; i++ {
		if _, err := gate.Report(inflight[i], false); err != nil {
			t.Fatal(err)
		}
	}

	now = now.Add(time.Second)
	probe, _, err := gate.Allow("k")
	if err != nil || !probe.Allowed() {
		t.Fatalf("probe: allowed=%v err=%v", probe.Allowed(), err)
	}
	if _, err := gate.Report(inflight[5], true); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(probe, false); err != nil {
		t.Fatal(err)
	}

	gate.mu.Lock()
	state := gate.keys["k"].state
	n := gate.keys["k"].n
	gate.mu.Unlock()
	if state != stateOpen {
		t.Fatalf("state = %v, want OPEN after real probe failure", state)
	}
	if n != 1 {
		t.Fatalf("n = %d, want 1", n)
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("want deny after failed probe")
	}
	if retryAfter != 2*time.Second {
		t.Fatalf("retryAfter = %v, want 2s for n=1", retryAfter)
	}
}

func TestProbePermit_ExpiredLeaseLateReportIgnored(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)

	now = now.Add(time.Second)
	probeA, _, err := gate.Allow("k")
	if err != nil || !probeA.Allowed() {
		t.Fatalf("probe A: allowed=%v err=%v", probeA.Allowed(), err)
	}
	now = now.Add(time.Second)
	probeB, _, err := gate.Allow("k")
	if err != nil || !probeB.Allowed() {
		t.Fatalf("probe B: allowed=%v err=%v", probeB.Allowed(), err)
	}
	if _, err := gate.Report(probeA, true); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	state := gate.keys["k"].state
	outstanding := gate.keys["k"].probeOutstanding
	gate.mu.Unlock()
	if state != stateHalfOpen {
		t.Fatalf("after stale A: state = %v, want HALF-OPEN", state)
	}
	if !outstanding {
		t.Fatal("after stale A: probe B must still be outstanding")
	}

	if _, err := gate.Report(probeB, false); err != nil {
		t.Fatal(err)
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("B's failure must reopen")
	}
	if retryAfter != 2*time.Second {
		t.Fatalf("retryAfter = %v, want 2s for n=1", retryAfter)
	}
}

func TestProbePermit_PreTripFailureAfterRecloseIgnored(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	stale, _, err := gate.Allow("k")
	if err != nil || !stale.Allowed() {
		t.Fatal(err)
	}
	trip(t, gate)

	now = now.Add(time.Second)
	probe, _, err := gate.Allow("k")
	if err != nil || !probe.Allowed() {
		t.Fatalf("probe: allowed=%v err=%v", probe.Allowed(), err)
	}
	if _, err := gate.Report(probe, true); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(stale, false); err != nil {
		t.Fatal(err)
	}

	gate.mu.Lock()
	credit := gate.keys["k"].credit
	state := gate.keys["k"].state
	n := gate.keys["k"].n
	gate.mu.Unlock()
	if state != stateClosed {
		t.Fatalf("state = %v, want CLOSED", state)
	}
	if credit != 5 {
		t.Fatalf("credit = %v, want B (stale failure must not subtract)", credit)
	}
	if n != 0 {
		t.Fatalf("n = %d, want 0", n)
	}
}

func TestProbePermit_ExpiredRecreatedKeyIgnored(t *testing.T) {
	gate, err := New(Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	stale, _, err := gate.Allow("k")
	if err != nil || !stale.Allowed() {
		t.Fatal(err)
	}
	now = now.Add(4 * time.Second)
	fresh, _, err := gate.Allow("k")
	if err != nil || !fresh.Allowed() {
		t.Fatalf("recreated: allowed=%v err=%v", fresh.Allowed(), err)
	}
	if _, err := gate.Report(stale, false); err != nil {
		t.Fatal(err)
	}

	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gen := gate.keys["k"].gen
	gate.mu.Unlock()
	if credit != 5 {
		t.Fatalf("credit = %v, want B (expired permit must not land)", credit)
	}
	if gen == stale.gen {
		t.Fatal("recreated entry must not reuse the expired permit generation")
	}
}

func TestProbePermit_ZeroPermitIgnored(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatal(err)
	}
	gate.mu.Lock()
	expireBefore := gate.keys["k"].expireAt
	gate.mu.Unlock()
	now = now.Add(time.Second)
	if _, err := gate.Report(Permit{}, false); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	state := gate.keys["k"].state
	expireAfter := gate.keys["k"].expireAt
	gate.mu.Unlock()
	if state != stateClosed {
		t.Fatalf("state = %v, want CLOSED", state)
	}
	if credit != 5 {
		t.Fatalf("credit = %v, want B", credit)
	}
	if !expireAfter.Equal(expireBefore) {
		t.Fatalf("expireAt moved from %v to %v; zero permit must not refresh TTL", expireBefore, expireAfter)
	}
}

// TestProbePermit_ReportSaysWhetherApplied: Report returns applied true only for a permit of the key's current
// generation, so a caller can tell a counted outcome (for example a probe that closed the gate) from an ignored one.
func TestProbePermit_ReportSaysWhetherApplied(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	closed, _, err := gate.Allow("k")
	if err != nil || !closed.Allowed() {
		t.Fatalf("closed admit: allowed=%v err=%v", closed.Allowed(), err)
	}
	if applied, err := gate.Report(closed, true); err != nil || !applied {
		t.Fatalf("current-generation report: applied=%v err=%v, want true", applied, err)
	}
	if applied, err := gate.Report(Permit{}, true); err != nil || applied {
		t.Fatalf("zero permit: applied=%v err=%v, want false", applied, err)
	}

	// Trip the key, then admit a probe and let its lease expire so a second probe replaces it.
	stale := make([]Permit, 0, 5)
	for i := 0; i < 5; i++ {
		p, _, allowErr := gate.Allow("k")
		if allowErr != nil || !p.Allowed() {
			t.Fatalf("admit %d: allowed=%v err=%v", i, p.Allowed(), allowErr)
		}
		stale = append(stale, p)
	}
	for _, p := range stale {
		if _, err := gate.Report(p, false); err != nil {
			t.Fatal(err)
		}
	}
	if applied, err := gate.Report(closed, false); err != nil || applied {
		t.Fatalf("pre-trip permit after trip: applied=%v err=%v, want false", applied, err)
	}
	now = now.Add(time.Second)
	probeA, _, err := gate.Allow("k")
	if err != nil || !probeA.Allowed() {
		t.Fatalf("probe A: allowed=%v err=%v", probeA.Allowed(), err)
	}
	now = now.Add(time.Second)
	probeB, _, err := gate.Allow("k")
	if err != nil || !probeB.Allowed() {
		t.Fatalf("probe B after expired lease: allowed=%v err=%v", probeB.Allowed(), err)
	}
	if applied, err := gate.Report(probeA, true); err != nil || applied {
		t.Fatalf("expired probe A success: applied=%v err=%v, want false", applied, err)
	}
	if applied, err := gate.Report(probeB, true); err != nil || !applied {
		t.Fatalf("active probe B success: applied=%v err=%v, want true", applied, err)
	}

	gate.Close()
	if applied, err := gate.Report(probeB, true); err == nil || applied {
		t.Fatalf("closed gate: applied=%v err=%v, want false and an error", applied, err)
	}
}
