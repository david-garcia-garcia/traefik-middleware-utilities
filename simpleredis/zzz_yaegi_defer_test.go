package simpleredis

import (
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_DeferRunsOnPanic is the evidence that overturns PR #29.
//
// PR #29 closed without deferred release, believing an interpreted defer would not
// run when Traefik recovered a panicking plugin. That is false on Yaegi v0.16.1
// (this module's pin, and Traefik v3.7.11's pin). Yaegi converts the panic into an
// Eval error rather than propagating a Go panic outward, but it still unwinds the
// interpreted frames and runs their defers on the way.
//
// turns==0 after each case means the deferred restore ran. If this test fails, do
// not re-derive the #29 assumption — measure again.
func TestYaegi_DeferRunsOnPanic(t *testing.T) {
	goPath := t.TempDir()
	writeGopathFile(t, goPath, "deferprobe", "deferprobe.go", yaegiDeferProbeSrc)

	cases := []struct{ name, call string }{
		{"explicit-panic", `deferprobe.ExplicitPanicWithDefer()`},
		{"interpreter-panic-errors-as", `deferprobe.InterpreterPanicWithDefer()`},
		{"nil-map-write", `deferprobe.NilMapPanicWithDefer()`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			interpreter := interp.New(interp.Options{GoPath: goPath})
			if err := interpreter.Use(stdlib.Symbols); err != nil {
				t.Fatalf("use stdlib: %v", err)
			}
			if _, err := interpreter.Eval(`import "deferprobe"`); err != nil {
				t.Fatalf("import: %v", err)
			}

			// Traefik shape: compiled code recovers a panic that escaped the interpreted plugin.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("recovered at compiled boundary: %v", r)
					}
				}()
				if _, err := interpreter.Eval(test.call); err != nil {
					t.Logf("Eval returned error: %v", err)
				}
			}()

			got, err := interpreter.Eval(`deferprobe.Turns()`)
			if err != nil {
				t.Fatalf("Turns: %v", err)
			}
			if got.Interface() != "0" {
				t.Fatalf("turns=%v after %s, want 0 (interpreted defer did not run; a deferred release would lose the in-use turn)",
					got.Interface(), test.name)
			}
		})
	}
}

const yaegiDeferProbeSrc = `package deferprobe

import (
	"errors"
	"strconv"
)

// turns mimics an in-use turn: taken before the risky call, returned by defer.
var turns int

// wrap is a package-local error struct, the shape errors.As is documented to panic on under Yaegi.
type wrap struct{ err error }

func (w wrap) Error() string { return w.err.Error() }
func (w wrap) Unwrap() error { return w.err }

var target = errors.New("target")

// ExplicitPanicWithDefer panics from interpreted code with a deferred restore pending.
func ExplicitPanicWithDefer() {
	turns++
	defer func() { turns-- }()
	panic("boom")
}

// InterpreterPanicWithDefer triggers the interpreter's own errors.As panic with a defer pending.
func InterpreterPanicWithDefer() {
	turns++
	defer func() { turns-- }()
	var dst wrap
	_ = errors.As(wrap{err: target}, &dst)
}

// NilMapPanicWithDefer panics inside a runtime operation with a defer pending.
func NilMapPanicWithDefer() {
	turns++
	defer func() { turns-- }()
	var m map[string]int
	m["x"] = 1
}

// Turns reports the counter so the compiled test can see whether defer ran.
func Turns() string { return strconv.Itoa(turns) }
`
