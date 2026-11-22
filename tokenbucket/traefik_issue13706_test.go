package tokenbucket

import (
	"context"
	"testing"
	"time"
)

const burst13706 = int64(3)

const key13706 = "client-1"

// countAdmits13706 calls allow until a deny and returns how many Status.Allowed were true.
// attempts is a cap so a limiter that never denies cannot loop forever.
func countAdmits13706(t *testing.T, attempts int, allow func() (Status, error)) int {
	t.Helper()
	admitted := 0
	for i := 0; i < attempts; i++ {
		status, err := allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i+1, err)
		}
		if !status.Allowed {
			return admitted
		}
		admitted++
	}
	t.Fatalf("limiter still admitting after %d Allows", attempts)
	return admitted
}

// newFrozenMemory13706 builds a Memory with maxDelay 0 and a frozen clock so refill cannot hide the burst cap.
func newFrozenMemory13706(t *testing.T, now time.Time) *Memory {
	t.Helper()
	limiter, err := NewMemory(1, burst13706, 0, testTTL)
	if err != nil {
		t.Fatal(err)
	}
	limiter.SetNowForTest(func() time.Time { return now })
	return limiter
}

// TestTraefik13706_MemoryScope asserts the documented Memory key space: one *Memory is one budget, two values are two.
func TestTraefik13706_MemoryScope(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	attempts := int(burst13706) + 5

	t.Run("sharedMemory", func(t *testing.T) {
		shared := newFrozenMemory13706(t, now)
		// Two routers that reuse one *Memory (the way to share a budget) drain the same key.
		routerA := countAdmits13706(t, attempts, func() (Status, error) {
			return shared.Allow(context.Background(), key13706)
		})
		routerB := countAdmits13706(t, attempts, func() (Status, error) {
			return shared.Allow(context.Background(), key13706)
		})
		total := routerA + routerB
		if routerA != int(burst13706) || routerB != 0 {
			t.Fatalf("shared *Memory: first caller admitted %d, second admitted %d, want %d then 0", routerA, routerB, burst13706)
		}
		if total != int(burst13706) {
			t.Fatalf("two callers sharing one *Memory admitted %d, want burst %d", total, burst13706)
		}
	})

	t.Run("separateMemory", func(t *testing.T) {
		// Documented per-instance behavior (traefik/traefik#13706): Traefik builds one in-memory limiter per router,
		// so N routers give N budgets. Each NewMemory is its own key space; two values admit 2*burst.
		routerA := newFrozenMemory13706(t, now)
		routerB := newFrozenMemory13706(t, now)
		admittedA := countAdmits13706(t, attempts, func() (Status, error) {
			return routerA.Allow(context.Background(), key13706)
		})
		admittedB := countAdmits13706(t, attempts, func() (Status, error) {
			return routerB.Allow(context.Background(), key13706)
		})
		total := admittedA + admittedB
		if admittedA != int(burst13706) || admittedB != int(burst13706) {
			t.Fatalf("separate *Memory: admitted %d and %d, want burst %d each", admittedA, admittedB, burst13706)
		}
		if total != 2*int(burst13706) {
			t.Fatalf("two separate *Memory values admitted %d, want 2*burst %d", total, 2*burst13706)
		}
	})
}
