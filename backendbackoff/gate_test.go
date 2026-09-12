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

func TestAllow_CanceledContext(t *testing.T) {
	gate := newTestGate(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, _, err := gate.Allow(ctx, "k")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if allowed {
		t.Fatal("canceled Allow must not admit")
	}
}

func TestReport_ConsecutiveFailuresTrip(t *testing.T) {
	gate := newTestGate(t)
	ctx := context.Background()
	for i := 0; i < defaultTripFailures; i++ {
		allowed, _, err := gate.Allow(ctx, "k")
		if err != nil || !allowed {
			t.Fatalf("admit %d: allowed=%v err=%v", i, allowed, err)
		}
		if err := gate.Report("k", false); err != nil {
			t.Fatal(err)
		}
	}
	allowed, retryAfter, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
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
	if _, _, err := gate.Allow(context.Background(), "k"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	allowed, _, err := gate.Allow(context.Background(), "k")
	if err != nil || !allowed {
		t.Fatalf("lost probe lease must admit: allowed=%v err=%v", allowed, err)
	}
}

func TestReport_OpenIgnored(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	if err := gate.Report("k", true); err != nil {
		t.Fatal(err)
	}
	allowed, retryAfter, err := gate.Allow(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("Report success while OPEN must not close")
	}
	if retryAfter != time.Second {
		t.Fatalf("retryAfter = %v, want 1s", retryAfter)
	}
}

func TestReport_SuccessCapsAtB(t *testing.T) {
	gate := newTestGate(t)
	if _, _, err := gate.Allow(context.Background(), "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", true); err != nil {
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
	if _, _, err := gate.Allow(context.Background(), "k"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if err := gate.Report("k", false); err != nil {
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
	ctx := context.Background()
	allowed, _, err := gate.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Fatal(err)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", true); err != nil {
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
	allowed, _, err := gate.Allow(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("idle OPEN key must drop and admit")
	}
	gate.mu.Lock()
	credit := gate.keys["k"].credit
	gate.mu.Unlock()
	if credit != 5 {
		t.Fatalf("credit = %v, want B", credit)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	allowed, _, err = gate.Allow(context.Background(), "k")
	if err != nil || !allowed {
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
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(2 * time.Second)
	allowed, _, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("still OPEN at 2s, cooldown 10s")
	}
	now = now.Add(2 * time.Second)
	allowed, _, err = gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("deny must have refreshed TTL; key must still be OPEN, not a dropped CLOSED")
	}
}

func TestAllow_ProbeAfterCooldown(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(time.Second)
	allowed, _, err := gate.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Fatalf("probe: allowed=%v err=%v", allowed, err)
	}
	allowed, retryAfter, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
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
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", true); err != nil {
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
	allowed, retryAfter, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
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
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	allowed, retryAfter, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
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
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Second)
	trip(t, gate)
	allowed, retryAfter, err := gate.Allow(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
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
	ctx := context.Background()
	trip(t, gate)
	now = now.Add(time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := gate.Report("k", true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	trip(t, gate)
	_, retryAfter, err := gate.Allow(ctx, "k")
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
	allowed, _, err := gate.Allow(context.Background(), "k")
	if !errors.Is(err, errClosed) {
		t.Fatalf("err = %v", err)
	}
	if allowed {
		t.Fatal("closed gate must not admit")
	}
	if err := gate.Report("k", true); !errors.Is(err, errClosed) {
		t.Fatalf("Report = %v", err)
	}
}

func TestReport_CanceledRequestStillRecords(t *testing.T) {
	gate := newTestGate(t)
	ctx, cancel := context.WithCancel(context.Background())
	allowed, _, err := gate.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Fatal(err)
	}
	cancel()
	if err := gate.Report("k", false); err != nil {
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
	ctx := context.Background()
	for i := 0; i < defaultTripFailures; i++ {
		if _, _, err := gate.Allow(ctx, "k"); err != nil {
			t.Fatal(err)
		}
		if err := gate.Report("k", false); err != nil {
			t.Fatal(err)
		}
	}
}
