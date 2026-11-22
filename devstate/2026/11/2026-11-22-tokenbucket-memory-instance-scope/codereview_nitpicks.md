# Nitpicks

1. [hard] Name for the scope — `tokenbucket/traefik_issue13706_test.go:54` — `sharedMemory` stores admit counts in `routerA`/`routerB`; this body adds and compares those ints, it does not hold routers
   Quote:
   ```
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
   ```
   Fix: Rename the sharedMemory counts to `admittedA`/`admittedB` (the role this body adds and asserts)
   Status: done
   Argument: renamed sharedMemory counts to admittedA/admittedB in tokenbucket/traefik_issue13706_test.go
2. [hard] Symmetry and consistency — `tokenbucket/traefik_issue13706_test.go:51` — sibling subtests spell the admit-count role as `routerA`/`routerB` then `admittedA`/`admittedB`, and reuse `routerA` for an int in one branch and a `*Memory` in the other
   Quote:
   ```
   t.Run("sharedMemory", func(t *testing.T) {
   	shared := newFrozenMemory13706(t, now)
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
   ```
   Fix: Use `admittedA`/`admittedB` for admit counts in both subtests; keep `routerA`/`routerB` only for the separate `*Memory` values
   Status: done
   Argument: sharedMemory now uses admittedA/admittedB; separateMemory still uses routerA/routerB for *Memory values
