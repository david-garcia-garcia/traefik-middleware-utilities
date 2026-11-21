package reclaim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

// TestYaegi_WatchPublishedDeliveredToFuncAny proves Watch delivers Published to func(any) under Yaegi.
func TestYaegi_WatchPublishedDeliveredToFuncAny(t *testing.T) {
	if raceDetectorOn {
		t.Skip("Yaegi v0.16.1 select races inside the interp on context cancel; Unit without -race still runs this")
	}
	goPath := t.TempDir()
	writeGopathReclaim(t, goPath)
	writeGopathFile(t, goPath, "aliasprobe", "watch.go", aliasWatchYaegiSrc)

	got := evalAliasprobe(t, goPath, `aliasprobe.RunWatchPublished()`)
	if got != "ok" {
		t.Fatalf("Watch Published under Yaegi: %q, want ok", got)
	}
}

// TestYaegi_OpenTypedCallExpressionReturnsT proves OpenTyped as a call expression under Yaegi.
func TestYaegi_OpenTypedCallExpressionReturnsT(t *testing.T) {
	if raceDetectorOn {
		t.Skip("Yaegi v0.16.1 select races inside the interp on context cancel; Unit without -race still runs this")
	}
	goPath := t.TempDir()
	writeGopathReclaim(t, goPath)
	writeGopathFile(t, goPath, "hookprobe", "typed.go", hookprobeTypedSrc)

	got := evalHookprobe(t, goPath, `hookprobe.RunOpenTyped()`)
	if got != "ok" {
		t.Fatalf("OpenTyped call expression: %q, want ok", got)
	}
}

// TestYaegi_GraceExpireDoesNotHang is the DestBranch hang: interpreted concurrent last-holder
// drop with positive grace. Before AfterFunc, Go 1.21.13 missed Close and parked waitGraceOrWake
// in interp._select. Fail in 3s, not the 5-minute package timeout.
func TestYaegi_GraceExpireDoesNotHang(t *testing.T) {
	if raceDetectorOn {
		t.Skip("Yaegi v0.16.1 select races inside the interp on context cancel; Unit without -race still runs this")
	}
	goPath := t.TempDir()
	writeGopathReclaim(t, goPath)
	writeGopathFile(t, goPath, "graceprobe", "expire.go", graceExpireHangSrc)

	done := make(chan string, 1)
	go func() {
		interpreter := interp.New(interp.Options{GoPath: goPath})
		if err := interpreter.Use(stdlib.Symbols); err != nil {
			done <- "use:" + err.Error()
			return
		}
		if _, err := interpreter.Eval(`import "graceprobe"`); err != nil {
			done <- "import:" + err.Error()
			return
		}
		evaluated, err := interpreter.Eval(`graceprobe.ConcurrentExpire()`)
		if err != nil {
			done <- "eval:" + err.Error()
			return
		}
		got, ok := evaluated.Interface().(string)
		if !ok {
			done <- "type"
			return
		}
		done <- got
	}()

	select {
	case got := <-done:
		if got != "ok" {
			t.Fatalf("yaegi grace expire: %q, want ok", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("interpreted concurrent grace expire hung")
	}
}

func evalAliasprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "aliasprobe"`); err != nil {
		t.Fatalf("import aliasprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
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

const aliasWatchYaegiSrc = `package aliasprobe

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

type client struct{ id string }

// RunWatchPublished stores Published in atomic.Value via func(any) and SetAlias.
func RunWatchPublished() string {
	tab := reclaim.New(reclaim.Config{Grace: 0})
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	var dest atomic.Value
	ctx := context.Background()
	tab.Watch(ctx, "alias:shared", (*client)(nil), func(published any) {
		notice, ok := published.(reclaim.Published)
		if !ok {
			return
		}
		dest.Store(&reclaim.Box{Value: notice.Value})
	})
	c := &client{id: "A"}
	if _, err := tab.OpenWithHooks(ctx, "owner", logger, func() (any, reclaim.Hooks, error) {
		return c, reclaim.Hooks{}, nil
	}); err != nil {
		return "open:" + err.Error()
	}
	if err := tab.SetAlias("owner", "alias:shared", "p", "g"); err != nil {
		return "alias:" + err.Error()
	}
	prev := dest.Load()
	boxed, ok := prev.(*reclaim.Box)
	if !ok || boxed.Value != c {
		return "value"
	}
	return "ok"
}
`

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

const hookprobeTypedSrc = `package hookprobe

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

type item struct {
	n int
}

// RunOpenTyped calls OpenTyped as a call expression in this package, which can name item.
func RunOpenTyped() string {
	tab := reclaim.New(reclaim.Config{Grace: 20 * time.Millisecond})
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stored, err := reclaim.OpenTyped[*item](ctx, tab, "k", logger, func() (any, reclaim.Hooks, error) {
		return &item{n: 7}, reclaim.Hooks{}, nil
	})
	if err != nil {
		return "open:" + err.Error()
	}
	if stored == nil || stored.n != 7 {
		return "value"
	}
	again, err := reclaim.OpenTyped[*item](ctx, tab, "k", logger, func() (any, reclaim.Hooks, error) {
		return &item{n: 8}, reclaim.Hooks{}, nil
	})
	if err != nil {
		return "bind:" + err.Error()
	}
	if again != stored {
		return "identity"
	}
	return "ok"
}
`

const graceExpireHangSrc = `package graceprobe

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

// ConcurrentExpire drops many keys at once so a missed grace timer cannot hide in a serial loop.
func ConcurrentExpire() string {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	tab := reclaim.New(reclaim.Config{Grace: time.Millisecond})
	for round := 0; round < 8; round++ {
		var wg sync.WaitGroup
		var misses atomic.Int32
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				var closes atomic.Int32
				ctx, cancel := context.WithCancel(context.Background())
				_, err := tab.Open(ctx, fmt.Sprintf("c%d-%d", round, n), logger, func() (any, error) {
					return n, nil
				}, reclaim.Hooks{
					Close: func() { closes.Add(1) },
				})
				if err != nil {
					misses.Add(1)
					return
				}
				cancel()
				deadline := time.Now().Add(2 * time.Second)
				for closes.Load() == 0 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if closes.Load() == 0 {
					misses.Add(1)
				}
			}(i)
		}
		wg.Wait()
		if misses.Load() > 0 {
			return fmt.Sprintf("concurrent-miss:round=%d misses=%d", round, misses.Load())
		}
	}
	return "ok"
}
`
