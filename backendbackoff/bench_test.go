package backendbackoff

import (
	"context"
	"testing"
	"time"
)

func BenchmarkAllowWarm(b *testing.B) {
	gate, err := New(Config{
		FailureRatio: defaultFailureRatio,
		TripFailures: defaultTripFailures,
		BaseCooldown: time.Second,
		MaxCooldown:  defaultMaxCooldown,
		Jitter:       0,
		TTL:          defaultTTL,
	})
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := gate.Allow(ctx, "k"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := gate.Allow(ctx, "k"); err != nil {
			b.Fatal(err)
		}
	}
}

const (
	allowWarmAllocs int64 = 1  // measured 0
	allowWarmBytes  int64 = 64 // measured 0 + 64
)

func skipAllocCeilingIfRace(t *testing.T) {
	t.Helper()
	if !raceDetectorOn {
		return
	}
	t.Skip("alloc ceilings measure dest non-race builds; the detector inflates B/op")
}

func allocExceedsCeiling(result testing.BenchmarkResult, maxAllocs, maxBytes int64) (allocsOver, bytesOver bool) {
	return result.AllocsPerOp() > maxAllocs, result.AllocedBytesPerOp() > maxBytes
}

func assertAllocCeiling(t *testing.T, name string, result testing.BenchmarkResult, maxAllocs, maxBytes int64) {
	t.Helper()
	allocs := result.AllocsPerOp()
	bytesPerOp := result.AllocedBytesPerOp()
	t.Logf("%s: %d allocs/op, %d B/op (ceilings %d allocs/op, %d B/op)", name, allocs, bytesPerOp, maxAllocs, maxBytes)
	allocsOver, bytesOver := allocExceedsCeiling(result, maxAllocs, maxBytes)
	if allocsOver {
		t.Errorf("%s: %d allocs/op exceeds Go 1.21 ceiling %d", name, allocs, maxAllocs)
	}
	if bytesOver {
		t.Errorf("%s: %d B/op exceeds Go 1.21 ceiling %d", name, bytesPerOp, maxBytes)
	}
}

func TestAllocAllowWarm(t *testing.T) {
	skipAllocCeilingIfRace(t)
	assertAllocCeiling(t, "warm Allow", testing.Benchmark(BenchmarkAllowWarm), allowWarmAllocs, allowWarmBytes)
}
