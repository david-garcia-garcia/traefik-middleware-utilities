package backendbackoff

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNew_ZeroConfigDefaults(t *testing.T) {
	gate, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if gate.cfg.FailureRatio != defaultFailureRatio {
		t.Fatalf("FailureRatio = %v, want %v", gate.cfg.FailureRatio, defaultFailureRatio)
	}
	if gate.cfg.TripFailures != defaultTripFailures {
		t.Fatalf("TripFailures = %d, want %d", gate.cfg.TripFailures, defaultTripFailures)
	}
	if gate.cfg.Jitter != defaultJitter {
		t.Fatalf("Jitter = %v, want %v", gate.cfg.Jitter, defaultJitter)
	}
}

func TestNew_RejectsInvalidRatio(t *testing.T) {
	if _, err := New(Config{FailureRatio: 1, TripFailures: 1, BaseCooldown: time.Second, TTL: time.Second}); !errors.Is(err, errRatio) {
		t.Fatalf("ratio 1: %v", err)
	}
	if _, err := New(Config{FailureRatio: -0.1, TripFailures: 1, BaseCooldown: time.Second, TTL: time.Second}); !errors.Is(err, errRatio) {
		t.Fatalf("ratio negative: %v", err)
	}
}

func TestReport_ConsecutiveFailuresTrip(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	for i := 0; i < defaultTripFailures; i++ {
		permit, _, err := gate.Allow("k")
		if err != nil || !permit.Allowed() {
			t.Fatalf("admit %d: allowed=%v err=%v", i, permit.Allowed(), err)
		}
		if _, err := gate.Report(permit, false); err != nil {
			t.Fatal(err)
		}
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("want deny after B failures")
	}
	if retryAfter != time.Second {
		t.Fatalf("retryAfter = %v, want 1s", retryAfter)
	}
}

func TestAllow_LostProbeLeaseAdmits(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	if _, _, err := gate.Allow("k"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatalf("lost probe lease must admit: allowed=%v err=%v", permit.Allowed(), err)
	}
}

func TestReport_OpenIgnored(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	gate.mu.Lock()
	openPermit := Permit{key: "k", gen: gate.keys["k"].gen}
	gate.mu.Unlock()
	if _, err := gate.Report(openPermit, true); err != nil {
		t.Fatal(err)
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("Report success while OPEN must not close")
	}
	if retryAfter != time.Second {
		t.Fatalf("retryAfter = %v, want 1s", retryAfter)
	}
}

func TestReport_SuccessCapsAtB(t *testing.T) {
	gate := newTestGate(t)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, true); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gate.mu.Unlock()
	if credit != 5 {
		t.Fatalf("credit = %v, want B", credit)
	}
}

func TestReport_AfterTTLStillRecords(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gate.mu.Unlock()
	if credit != 4 {
		t.Fatalf("credit = %v, want 4 after Report past TTL", credit)
	}
}

func TestReport_SuccessCredits(t *testing.T) {
	gate := newTestGate(t)
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, true); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gate.mu.Unlock()
	want := float64(defaultTripFailures) - 1 + gate.successCredit()
	if credit != want {
		t.Fatalf("credit = %v, want %v", credit, want)
	}
}

func TestAllow_IdleKeyPresumedHealthy(t *testing.T) {
	gate, err := New(Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: 10 * time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(4 * time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if !permit.Allowed() {
		t.Fatal("idle OPEN key must drop and admit")
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gate.mu.Unlock()
	if credit != 5 {
		t.Fatalf("credit = %v, want B", credit)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	permit, _, err = gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatal("one failure after idle drop must not re-OPEN")
	}
}

func TestAllow_DenyRefreshesTTL(t *testing.T) {
	gate, err := New(Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: 10 * time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(2 * time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("still OPEN at 2s, cooldown 10s")
	}
	now = now.Add(2 * time.Second)
	permit, _, err = gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("deny must have refreshed TTL; key must still be OPEN, not a dropped CLOSED")
	}
}

func TestAllow_ProbeAfterCooldown(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatalf("probe: allowed=%v err=%v", permit.Allowed(), err)
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("second Allow during probe must deny")
	}
	if retryAfter != time.Second {
		t.Fatalf("probe lease retryAfter = %v, want 1s", retryAfter)
	}
}

func TestReport_ProbeSuccessRetainsN(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	permit, _, err = gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, true); err != nil {
		t.Fatal(err)
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	state := gate.keys["k"].state
	gate.mu.Unlock()
	if credit != 5 {
		t.Fatalf("probe success credit = %v, want B", credit)
	}
	if state != stateClosed {
		t.Fatalf("state = %v, want CLOSED", state)
	}
	trip(t, gate)
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("want OPEN")
	}
	if retryAfter != 2*time.Second {
		t.Fatalf("retained n=1 cooldown = %v, want 2s", retryAfter)
	}
}

func TestReport_ProbeFailureIncrementsN(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("want OPEN after failed probe")
	}
	if retryAfter != 2*time.Second {
		t.Fatalf("n=1 cooldown = %v, want 2s", retryAfter)
	}
}

func TestAllow_NResetsAfterMaxClosed(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	permit, _, err = gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Second)
	trip(t, gate)
	permit, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if permit.Allowed() {
		t.Fatal("want OPEN")
	}
	if retryAfter != time.Second {
		t.Fatalf("reset n cooldown = %v, want 1s", retryAfter)
	}
}

func TestAllow_EarlyRetripKeepsN(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	permit, _, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	permit, _, err = gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Report(permit, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	trip(t, gate)
	_, retryAfter, err := gate.Allow("k")
	if err != nil {
		t.Fatal(err)
	}
	if retryAfter != 2*time.Second {
		t.Fatalf("kept n cooldown = %v, want 2s", retryAfter)
	}
}

func TestClose_AllowErrors(t *testing.T) {
	gate := newTestGate(t)
	gate.Close()
	permit, _, err := gate.Allow("k")
	if !errors.Is(err, errClosed) {
		t.Fatalf("err = %v", err)
	}
	if permit.Allowed() {
		t.Fatal("closed gate must not admit")
	}
	if _, err := gate.Report(Permit{}, true); !errors.Is(err, errClosed) {
		t.Fatalf("Report = %v", err)
	}
}

func TestReport_CanceledRequestStillRecords(t *testing.T) {
	gate := newTestGate(t)
	_, cancel := context.WithCancel(context.Background())
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatal(err)
	}
	cancel()
	if _, err := gate.Report(permit, false); err != nil {
		t.Fatal(err)
	}
}

// newTestGate is a jitter-off gate with packaged trip and cooldown knobs.
func newTestGate(t *testing.T) *Gate {
	t.Helper()
	gate, err := New(Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return gate
}

// trip Reports B consecutive failures so the next Allow is OPEN.
func trip(t *testing.T, gate *Gate) {
	t.Helper()
	for i := 0; i < defaultTripFailures; i++ {
		permit, _, err := gate.Allow("k")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := gate.Report(permit, false); err != nil {
			t.Fatal(err)
		}
	}
}
