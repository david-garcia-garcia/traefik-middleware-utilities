package backendbackoff

import (
	"errors"
	"testing"
	"time"
)

func TestTripped_FollowsTripProbeAndClose(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	if mustTrippedTripped(t, gate) {
		t.Fatal("an unseen key is not tripped")
	}
	if len(gate.keys) != 0 {
		t.Fatalf("Tripped stored %d keys, want 0", len(gate.keys))
	}
	trip(t, gate)
	if !mustTrippedTripped(t, gate) {
		t.Fatal("OPEN key must be tripped")
	}
	now = now.Add(time.Second)
	if !mustTrippedTripped(t, gate) {
		t.Fatal("OPEN key past its cooldown is still tripped until a probe closes it")
	}
	probe, _, err := gate.Allow("k")
	if err != nil || !probe.Allowed() {
		t.Fatalf("probe: allowed=%v err=%v", probe.Allowed(), err)
	}
	if !mustTrippedTripped(t, gate) {
		t.Fatal("HALF-OPEN key must be tripped")
	}
	if applied, err := gate.Report(probe, true); err != nil || !applied {
		t.Fatalf("probe report: applied=%v err=%v", applied, err)
	}
	if mustTrippedTripped(t, gate) {
		t.Fatal("a probe success closes the key")
	}
}

func TestTripped_ProbeFailureStaysTripped(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)
	now = now.Add(time.Second)
	probe, _, err := gate.Allow("k")
	if err != nil || !probe.Allowed() {
		t.Fatalf("probe: allowed=%v err=%v", probe.Allowed(), err)
	}
	if _, err := gate.Report(probe, false); err != nil {
		t.Fatal(err)
	}
	if !mustTrippedTripped(t, gate) {
		t.Fatal("a failed probe reopens the key")
	}
}

func TestTripped_FalseOnceIdleTTLElapsed(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	trip(t, gate)

	now = now.Add(time.Minute - time.Second)
	if !mustTrippedTripped(t, gate) {
		t.Fatal("key within its idle TTL must be tripped")
	}
	// Tripped did not refresh the deadline: one more second drops the key.
	now = now.Add(time.Second)
	if mustTrippedTripped(t, gate) {
		t.Fatal("key idle for TTL is not tripped")
	}
	permit, _, err := gate.Allow("k")
	if err != nil || !permit.Allowed() {
		t.Fatalf("idle key admits: allowed=%v err=%v", permit.Allowed(), err)
	}
	if permit.Probe() {
		t.Fatal("the first admit after an idle drop is a CLOSED admit, not a probe")
	}
}

func TestTripped_AfterCloseErrors(t *testing.T) {
	gate := newTestGate(t)
	gate.Close()
	tripped, err := gate.Tripped("k")
	if !errors.Is(err, errClosed) {
		t.Fatalf("err = %v, want errClosed", err)
	}
	if tripped {
		t.Fatal("closed gate must report false")
	}
}

func TestPermit_ProbeOnlyForHalfOpenAdmits(t *testing.T) {
	gate := newTestGate(t)
	now := time.Unix(1_700_000_000, 0)
	gate.SetNowForTest(func() time.Time { return now })

	if (Permit{}).Probe() {
		t.Fatal("zero permit is not a probe")
	}
	closed, _, err := gate.Allow("k")
	if err != nil || !closed.Allowed() {
		t.Fatalf("closed admit: allowed=%v err=%v", closed.Allowed(), err)
	}
	if closed.Probe() {
		t.Fatal("a CLOSED admit is not a probe")
	}
	if _, err := gate.Report(closed, false); err != nil {
		t.Fatal(err)
	}
	trip(t, gate)
	denied, _, err := gate.Allow("k")
	if err != nil || denied.Allowed() {
		t.Fatalf("OPEN deny: allowed=%v err=%v", denied.Allowed(), err)
	}
	if denied.Probe() {
		t.Fatal("a deny is not a probe")
	}
	now = now.Add(time.Second)
	probe, _, err := gate.Allow("k")
	if err != nil || !probe.Allowed() || !probe.Probe() {
		t.Fatalf("OPEN to HALF-OPEN: allowed=%v probe=%v err=%v", probe.Allowed(), probe.Probe(), err)
	}
	// The lease is BaseCooldown; once it lapses the next Allow takes over the probe slot.
	now = now.Add(time.Second)
	replacement, _, err := gate.Allow("k")
	if err != nil || !replacement.Allowed() || !replacement.Probe() {
		t.Fatalf("replacement probe: allowed=%v probe=%v err=%v", replacement.Allowed(), replacement.Probe(), err)
	}
	if _, err := gate.Report(replacement, true); err != nil {
		t.Fatal(err)
	}
	after, _, err := gate.Allow("k")
	if err != nil || !after.Allowed() || after.Probe() {
		t.Fatalf("after close: allowed=%v probe=%v err=%v", after.Allowed(), after.Probe(), err)
	}
}

// mustTrippedTripped calls Tripped("k") and fails the test on error.
func mustTrippedTripped(t *testing.T, gate *Gate) bool {
	t.Helper()
	tripped, err := gate.Tripped("k")
	if err != nil {
		t.Fatal(err)
	}
	return tripped
}
