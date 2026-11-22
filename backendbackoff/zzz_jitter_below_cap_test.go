package backendbackoff

import (
	"math/rand"
	"testing"
	"time"
)

const (
	jitterBelowCapSamples  = 100000
	jitterBelowCapMaxShare = 0.001 // 0.1 %
	jitterBelowCapSeed     = 1
	jitterBelowCapAtN      = 10
)

// TestJitterBelowCap proves Decision 3 cases (a)-(d) for pre-bounded cooldown jitter.
func TestJitterBelowCap(t *testing.T) {
	t.Run("a_at_cap", testJitterBelowCapAtCap)
	t.Run("b_near_cap", testJitterBelowCapNearCap)
	t.Run("c_below_cap", testJitterBelowCapBelowCap)
	t.Run("d_jitter_zero", testJitterBelowCapZeroJitter)
}

// testJitterBelowCapAtCap fails against the old clamp: that piled ~50 % onto MaxCooldown.
func testJitterBelowCapAtCap(t *testing.T) {
	gate := newGateJitterBelowCap(t, configJitterBelowCap(defaultBaseCooldown, defaultJitter))
	samples := drawCooldownsJitterBelowCap(gate, jitterBelowCapAtN, jitterBelowCapSamples)
	assertSpreadJitterBelowCap(t, samples, 9*time.Second, gate.cfg.MaxCooldown)
}

// testJitterBelowCapNearCap fails against the old clamp: Base 9.5s overshot onto Max.
func testJitterBelowCapNearCap(t *testing.T) {
	gate := newGateJitterBelowCap(t, configJitterBelowCap(9500*time.Millisecond, defaultJitter))
	samples := drawCooldownsJitterBelowCap(gate, 0, jitterBelowCapSamples)
	assertSpreadJitterBelowCap(t, samples, 8550*time.Millisecond, gate.cfg.MaxCooldown)
}

// testJitterBelowCapBelowCap keeps the old ±Jitter band when the +J side is still under Max.
func testJitterBelowCapBelowCap(t *testing.T) {
	gate := newGateJitterBelowCap(t, configJitterBelowCap(defaultBaseCooldown, defaultJitter))
	samples := drawCooldownsJitterBelowCap(gate, 0, jitterBelowCapSamples)
	lo, hi := minMaxJitterBelowCap(samples)
	bandLo := 900 * time.Millisecond
	bandHi := 1100 * time.Millisecond
	if lo < bandLo || hi >= bandHi {
		t.Fatalf("below-cap samples [%v, %v], want in [%v, %v)", lo, hi, bandLo, bandHi)
	}
	if lo >= defaultBaseCooldown || hi <= defaultBaseCooldown {
		t.Fatalf("below-cap samples [%v, %v] did not spread around wait 1s", lo, hi)
	}
}

// testJitterBelowCapZeroJitter is the control: Jitter 0 returns wait exactly.
func testJitterBelowCapZeroJitter(t *testing.T) {
	atCap := newGateJitterBelowCap(t, configJitterBelowCap(defaultBaseCooldown, 0))
	below := newGateJitterBelowCap(t, configJitterBelowCap(defaultBaseCooldown, 0))
	if got := atCap.cooldownDuration(jitterBelowCapAtN); got != defaultMaxCooldown {
		t.Fatalf("Jitter 0 at cap = %v, want 10s", got)
	}
	if got := below.cooldownDuration(0); got != defaultBaseCooldown {
		t.Fatalf("Jitter 0 below cap = %v, want 1s", got)
	}
}

// configJitterBelowCap is a valid Config with the cooldown knobs this file varies.
func configJitterBelowCap(base time.Duration, jitter float64) Config {
	return Config{
		FailureRatio: defaultFailureRatio,
		TripFailures: defaultTripFailures,
		BaseCooldown: base,
		MaxCooldown:  defaultMaxCooldown,
		Jitter:       jitter,
		TTL:          defaultTTL,
	}
}

// newGateJitterBelowCap builds a gate and seeds rng so draws are repeatable.
func newGateJitterBelowCap(t *testing.T, cfg Config) *Gate {
	t.Helper()
	gate, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	gate.rng = rand.New(rand.NewSource(jitterBelowCapSeed)) //nolint:gosec // G404: test seed, not a secret
	return gate
}

// drawCooldownsJitterBelowCap records n-th-trip cooldownDuration samples.
func drawCooldownsJitterBelowCap(gate *Gate, n, count int) []time.Duration {
	samples := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		samples[i] = gate.cooldownDuration(n)
	}
	return samples
}

// assertSpreadJitterBelowCap requires every sample in [lower, maxCooldown) and no 0.1 % pile-up.
func assertSpreadJitterBelowCap(t *testing.T, samples []time.Duration, lower, maxCooldown time.Duration) {
	t.Helper()
	counts := make(map[time.Duration]int, len(samples))
	for i, sample := range samples {
		if sample < lower || sample >= maxCooldown {
			t.Fatalf("sample %d = %v, want in [%v, %v) (old clamp returned MaxCooldown)", i, sample, lower, maxCooldown)
		}
		counts[sample]++
	}
	mode := samples[0]
	modeCount := 0
	for sample, count := range counts {
		if count > modeCount {
			mode = sample
			modeCount = count
		}
	}
	share := float64(modeCount) / float64(len(samples))
	if share > jitterBelowCapMaxShare {
		t.Fatalf("value %v holds %.2f%% of samples, want <= 0.1%% (old clamp held ~50%% at Max)", mode, share*100)
	}
}

// minMaxJitterBelowCap returns the smallest and largest durations in samples.
func minMaxJitterBelowCap(samples []time.Duration) (lo, hi time.Duration) {
	lo = samples[0]
	hi = samples[0]
	for _, sample := range samples[1:] {
		if sample < lo {
			lo = sample
		}
		if sample > hi {
			hi = sample
		}
	}
	return lo, hi
}
