package windowcounter

import (
	"fmt"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_BufferedShareSleepDoesNotHang is the DestBranch hang: interpreted BufferedShare
// Sleep/Close. Before AfterFunc, Go 1.21.13 parked Wait in evalTakeprobe and flushLoop in
// interp._select. Fail in 3s, not the 5-minute package timeout.
func TestYaegi_BufferedShareSleepDoesNotHang(t *testing.T) {
	_, addr := startTestFakeRedis(t)
	goPath := t.TempDir()
	writeGopathWindowcounter(t, goPath)
	writeGopathFile(t, goPath, "takeprobe", "roundtrip.go", takeprobeSrc)

	done := make(chan string, 1)
	go func() {
		interpreter := interp.New(interp.Options{GoPath: goPath})
		if err := interpreter.Use(stdlib.Symbols); err != nil {
			done <- "use:" + err.Error()
			return
		}
		if _, err := interpreter.Eval(`import "takeprobe"`); err != nil {
			done <- "import:" + err.Error()
			return
		}
		expr := fmt.Sprintf(`takeprobe.BufferedShare(%q, %q)`, addr, t.Name())
		evaluated, err := interpreter.Eval(expr)
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
			t.Fatalf("yaegi buffered share: %q, want ok", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("interpreted BufferedShare hung in Sleep/Close")
	}
}

// TestYaegi_MethodFlushStopDoesNotHang is the isolated limiter flush shape: interpreted method
// goroutine, ticker+stop select, close, WaitGroup.Wait. Windowcounter no longer uses this path;
// the test stays so a reintroduction hangs here in 3s.
func TestYaegi_MethodFlushStopDoesNotHang(t *testing.T) {
	goPath := t.TempDir()
	writeGopathFile(t, goPath, "tickprobe", "tickprobe.go", tickprobeFlushSrc)

	done := make(chan string, 1)
	go func() {
		interpreter := interp.New(interp.Options{GoPath: goPath})
		if err := interpreter.Use(stdlib.Symbols); err != nil {
			done <- "use:" + err.Error()
			return
		}
		if _, err := interpreter.Eval(`import "tickprobe"`); err != nil {
			done <- "import:" + err.Error()
			return
		}
		evaluated, err := interpreter.Eval(`tickprobe.Run()`)
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
			t.Fatalf("yaegi tickprobe: %q, want ok", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("interpreted ticker+stop select hung on WaitGroup.Wait")
	}
}

const tickprobeFlushSrc = `package tickprobe

import (
	"sync"
	"time"
)

type flusher struct {
	ticker *time.Ticker
	stop   chan struct{}
	wg     sync.WaitGroup
}

func (l *flusher) start() {
	l.stop = make(chan struct{})
	l.ticker = time.NewTicker(time.Hour)
	l.wg.Add(1)
	go l.flushLoop(l.ticker, l.stop)
}

func (l *flusher) flushLoop(ticker *time.Ticker, stop <-chan struct{}) {
	defer l.wg.Done()
	for {
		select {
		case <-ticker.C:
		case <-stop:
			return
		}
	}
}

func (l *flusher) stopWait() {
	close(l.stop)
	l.wg.Wait()
	l.ticker.Stop()
}

func Run() string {
	l := &flusher{}
	l.start()
	l.stopWait()
	return "ok"
}
`
