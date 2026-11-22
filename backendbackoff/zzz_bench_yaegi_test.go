package backendbackoff

import (
	"fmt"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// runYaegiAllowBench interprets the gatebench probe and times one Eval of loopExpr(b.N) after a warm-up Eval of loopExpr(1).
// setupExpr, when not empty, runs once before the warm-up and is not timed.
func runYaegiAllowBench(b *testing.B, setupExpr string, loopExpr func(count int) string) {
	b.Helper()
	goPath := b.TempDir()
	writeGopathBackendbackoff(b, goPath)
	// Test-only prefill hook, written into the interpreter's copy of the package only.
	writeGopathFile(b, goPath, "github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff", "zz_bench_hooks.go", gateBenchHooksSrc)
	writeGopathFile(b, goPath, "gatebench", "loop.go", gatebenchSrc)

	// Interpreter with stdlib only (no unsafe), as Traefik loads a plugin.
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		b.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "gatebench"`); err != nil {
		b.Fatalf("import gatebench: %v", err)
	}
	if setupExpr != "" {
		evaluated, err := interpreter.Eval(setupExpr)
		if err != nil {
			b.Fatalf("setup: %v", err)
		}
		if got := evaluated.Interface().(string); got != "ok" {
			b.Fatalf("setup = %q, want ok", got)
		}
	}
	if _, err := interpreter.Eval(loopExpr(1)); err != nil {
		b.Fatalf("warmup: %v", err)
	}

	// One timed Eval runs b.N Allow calls inside the interpreter.
	b.ResetTimer()
	evaluated, err := interpreter.Eval(loopExpr(b.N))
	if err != nil {
		b.Fatalf("eval loop: %v", err)
	}
	b.StopTimer()
	if got := evaluated.Interface().(string); got != "ok" {
		b.Fatalf("loop = %q, want ok", got)
	}
}

// BenchmarkYaegiAllow measures interpreted new-key admission against a full table of live keys.
func BenchmarkYaegiAllow(b *testing.B) {
	b.Run("new-key-full-table", func(b *testing.B) {
		runYaegiAllowBench(b, fmt.Sprintf(`gatebench.SetupFullTable(%d)`, maxMemorySources), func(count int) string {
			return fmt.Sprintf(`gatebench.NewKeyLoop(%d)`, count)
		})
	})
}

// gatebenchSrc is the interpreted probe: Setup fills the store; NewKeyLoop Allows never-seen keys.
const gatebenchSrc = `package gatebench

import (
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff"
)

// benchGate and benchNewKeys are the state SetupFullTable leaves for the timed loop.
var (
	benchGate    *backendbackoff.Gate
	benchNewKeys int
)

// SetupFullTable fills the store with count live keys on a frozen clock for NewKeyLoop.
func SetupFullTable(count int) string {
	gate, err := backendbackoff.New(backendbackoff.Config{
		FailureRatio: 0.30,
		TripFailures: 5,
		BaseCooldown: time.Second,
		MaxCooldown:  10 * time.Second,
		Jitter:       0,
		TTL:          time.Minute,
	})
	if err != nil {
		return "new:" + err.Error()
	}
	now := time.Unix(1700000000, 0)
	gate.SetNowForTest(func() time.Time { return now })
	backendbackoff.PrefillForBench(gate, "live:", count, now)
	benchGate = gate
	return "ok"
}

// NewKeyLoop runs count Allow calls, each on a never-seen key, so every one evicts a live key.
func NewKeyLoop(count int) string {
	for i := 0; i < count; i++ {
		benchNewKeys++
		p, _, err := benchGate.Allow("new:" + strconv.Itoa(benchNewKeys))
		if err != nil {
			return "allow:" + err.Error()
		}
		if !p.Allowed() {
			return "denied"
		}
	}
	return "ok"
}
`

// gateBenchHooksSrc is a test-only file written into the interpreter's copy of package backendbackoff (never into the
// product package): it prefills a Gate without paying Allow's new-key cost.
const gateBenchHooksSrc = `package backendbackoff

import (
	"strconv"
	"time"
)

// PrefillForBench stores count CLOSED keys named prefix0..prefix(count-1) that expire one TTL after now.
func PrefillForBench(g *Gate, prefix string, count int, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	expireAt := now.Add(g.cfg.TTL)
	for i := 0; i < count; i++ {
		entry := &memEntry{
			credit:      g.budget,
			state:       stateClosed,
			closedSince: now,
			expireAt:    expireAt,
			key:         prefix + strconv.Itoa(i),
		}
		g.bumpGen(entry)
		g.storeLocked(entry)
	}
}
`
