package reclaim

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestTable_OpenWithHooksHonoursEnforceFromCreate(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	closeEntered := make(chan struct{})
	releaseClose := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-releaseClose:
		default:
			close(releaseClose)
		}
	})
	var createWhileCloseBlocked atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctx, "k", recLogger(h), func() (any, Hooks, error) {
		return firstIncarnation, Hooks{
			Close:                  func() { close(closeEntered); <-releaseClose },
			EnforceCloseBeforeOpen: true,
		}, nil
	}); err != nil {
		t.Fatalf("open 1: %v", err)
	}
	cancel()
	<-closeEntered

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	opened := make(chan error, 1)
	go func() {
		_, err := tab.Open(ctx2, "k", recLogger(h), func() (any, error) {
			createWhileCloseBlocked.Store(true)
			return nextIncarnation, nil
		}, Hooks{})
		opened <- err
	}()
	select {
	case err := <-opened:
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
		t.Fatalf("second Open returned before Close returned: %v", err)
	case <-time.After(200 * time.Millisecond):
		if createWhileCloseBlocked.Load() {
			t.Fatal("create of incarnation 2 ran while Close of 1 was blocked")
		}
	}
	close(releaseClose)
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
	case <-time.After(waitBudget):
		t.Fatal("second Open did not return after Close returned")
	}
	if !createWhileCloseBlocked.Load() {
		t.Fatal("second Open did not create after Close returned")
	}
}

func TestTable_OpenWithHooksLaterOpenBindsWithoutCreate(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, err := tab.OpenWithHooks(ctx, "a", recLogger(h), func() (any, Hooks, error) {
		return life, testLifeHooks(life), nil
	})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	created := 0
	second, err := tab.Open(ctx, "a", recLogger(h), func() (any, error) {
		created++
		t.Error("create ran again")
		return nil, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if first != second || first != life {
		t.Fatalf("bind value %v %v, want the same lifecycle", first, second)
	}
	if created != 0 {
		t.Fatalf("create ran %d times on the later Open", created)
	}
}

func TestOpenTypedReturnsTypedValue(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := OpenTyped[*lifecycle](ctx, tab, "a", recLogger(h), func() (any, Hooks, error) {
		return life, testLifeHooks(life), nil
	})
	if err != nil {
		t.Fatalf("OpenTyped: %v", err)
	}
	if got != life {
		t.Fatalf("OpenTyped value %v, want the created lifecycle", got)
	}
}

func TestOpenTypedSingletonIdentity(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	created := 0
	create := func() (any, Hooks, error) {
		created++
		return life, testLifeHooks(life), nil
	}
	first, err := OpenTyped[*lifecycle](ctx, tab, "a", recLogger(h), create)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, err := OpenTyped[*lifecycle](ctx, tab, "a", recLogger(h), create)
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if first != second || first != life {
		t.Fatalf("identity %v %v, want the same lifecycle", first, second)
	}
	if created != 1 {
		t.Fatalf("create ran %d times, want 1", created)
	}
}

func TestOpenTypedTypeMismatchReturnsZeroT(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := OpenTyped[*lifecycle](ctx, tab, "a", recLogger(h), func() (any, Hooks, error) {
		return "wrong-type", Hooks{}, nil
	})
	if got != nil {
		t.Fatalf("mismatch value %v, want nil zero", got)
	}
	want := fmt.Sprintf("reclaim: open %q: want %T, got %T", "a", (*lifecycle)(nil), "wrong-type")
	if err == nil || err.Error() != want {
		t.Fatalf("mismatch err %v, want %s", err, want)
	}
}

func TestTable_OpenNilCreateKeepsErrorPrecedence(t *testing.T) {
	logger := recLogger(&recHandler{})
	create := func() (any, error) { return ending(1, nil), nil }

	var nilTable *Table
	_, err := nilTable.Open(context.Background(), "a", logger, nil, Hooks{})
	wantTable := `reclaim: open "a": nil table`
	if err == nil || err.Error() != wantTable {
		t.Fatalf("nil table + nil create: %v, want %s", err, wantTable)
	}

	tab := New(Config{Grace: time.Millisecond})
	_, err = tab.Open(context.Background(), "a", nil, nil, Hooks{})
	wantLogger := `reclaim: open "a": nil logger`
	if err == nil || err.Error() != wantLogger {
		t.Fatalf("nil logger + nil create: %v, want %s", err, wantLogger)
	}

	_, err = tab.Open(context.Background(), "a", logger, nil, Hooks{})
	wantCreate := `reclaim: create "a": nil create`
	if err == nil || err.Error() != wantCreate {
		t.Fatalf("nil create: %v, want %s", err, wantCreate)
	}

	_, err = tab.Open(context.Background(), "a", logger, create, Hooks{})
	if err != nil {
		t.Fatalf("valid Open after precedence checks: %v", err)
	}
}

func TestTable_OpenWithHooksNilArgsMatchOpenErrors(t *testing.T) {
	logger := recLogger(&recHandler{})
	var nilTable *Table
	_, err := nilTable.OpenWithHooks(context.Background(), "a", logger, nil)
	wantTable := `reclaim: open "a": nil table`
	if err == nil || err.Error() != wantTable {
		t.Fatalf("nil table: %v, want %s", err, wantTable)
	}

	tab := New(Config{Grace: time.Millisecond})
	_, err = tab.OpenWithHooks(context.Background(), "a", nil, nil)
	wantLogger := `reclaim: open "a": nil logger`
	if err == nil || err.Error() != wantLogger {
		t.Fatalf("nil logger: %v, want %s", err, wantLogger)
	}

	_, err = tab.OpenWithHooks(context.Background(), "a", logger, nil)
	wantCreate := `reclaim: create "a": nil create`
	if err == nil || err.Error() != wantCreate {
		t.Fatalf("nil create: %v, want %s", err, wantCreate)
	}
}
