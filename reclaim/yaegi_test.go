package reclaim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_CreateAnyTypeSwitchDoesNotMatch proves Yaegi v0.16.1 synthesizes a create
// func() (any, error) return with no methods, so a type-switch to Sleep does not match.
func TestYaegi_CreateAnyTypeSwitchDoesNotMatch(t *testing.T) {
	goPath := t.TempDir()
	writeGopathReclaim(t, goPath)
	writeGopathFile(t, goPath, "hookprobe", "check.go", hookprobeCheckSrc)

	got := evalHookprobe(t, goPath, `hookprobe.CheckCreateSwitch()`)
	if got != "no" {
		t.Fatalf("type-switch on create any: %q, want no", got)
	}
}

// TestYaegi_OpenHooksRunSleepWakeClose proves Open Hooks funcs run under the interpreter.
func TestYaegi_OpenHooksRunSleepWakeClose(t *testing.T) {
	if raceDetectorOn {
		t.Skip("Yaegi v0.16.1 select races inside the interp on context cancel; Unit without -race still runs this")
	}
	goPath := t.TempDir()
	writeGopathReclaim(t, goPath)
	writeGopathFile(t, goPath, "hookprobe", "run.go", hookprobeRunSrc)

	got := evalHookprobe(t, goPath, `hookprobe.RunHooks()`)
	var sleeps, wakes, closes int
	if _, err := fmt.Sscanf(got, "%d %d %d", &sleeps, &wakes, &closes); err != nil {
		t.Fatalf("hook counts %q: %v", got, err)
	}
	if sleeps < 1 || wakes < 1 || closes < 1 {
		t.Fatalf("hook counts %q, want each at least 1", got)
	}
}

// evalHookprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalHookprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "hookprobe"`); err != nil {
		t.Fatalf("import hookprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathReclaim copies non-test reclaim sources into a GOPATH module tree.
func writeGopathReclaim(t *testing.T, goPath string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller path")
	}
	srcDir := filepath.Dir(thisFile)
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "reclaim")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		t.Fatal("no reclaim sources copied into GOPATH")
	}
}

// writeGopathFile writes one interpreted package file under GOPATH/src/<pkg>.
func writeGopathFile(t *testing.T, goPath, pkg, name, src string) {
	t.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}

const hookprobeCheckSrc = `package hookprobe

type sleeper interface {
	Sleep()
}

type life struct{}

func (l *life) Sleep() {}

// CheckCreateSwitch returns "no" when a create any return does not match sleeper.
func CheckCreateSwitch() string {
	create := func() (any, error) { return &life{}, nil }
	value, err := create()
	if err != nil {
		return "err"
	}
	switch value.(type) {
	case sleeper:
		return "match"
	default:
		return "no"
	}
}
`

const hookprobeRunSrc = `package hookprobe

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

// RunHooks drives one incarnation through sleep, wake, and close via Open Hooks.
func RunHooks() string {
	var sleeps, wakes, closes atomic.Int32
	tab := reclaim.New(reclaim.Config{Grace: 20 * time.Millisecond})
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx, cancel := context.WithCancel(context.Background())
	_, err := tab.Open(ctx, "k", logger, func() (any, error) {
		return 1, nil
	}, reclaim.Hooks{
		Sleep:                  func() { sleeps.Add(1) },
		Wake:                   func() { wakes.Add(1) },
		Close:                  func() { closes.Add(1) },
		EnforceCloseBeforeOpen: true,
	})
	if err != nil {
		return "open:" + err.Error()
	}
	cancel()
	waitCount(&sleeps)
	ctx2, cancel2 := context.WithCancel(context.Background())
	_, err = tab.Open(ctx2, "k", logger, func() (any, error) {
		return nil, fmt.Errorf("create ran again")
	}, reclaim.Hooks{})
	if err != nil {
		return "reopen:" + err.Error()
	}
	waitCount(&wakes)
	cancel2()
	waitCount(&closes)
	return fmt.Sprintf("%d %d %d", sleeps.Load(), wakes.Load(), closes.Load())
}

func waitCount(hookCount *atomic.Int32) {
	deadline := time.Now().Add(2 * time.Second)
	for hookCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}
`
