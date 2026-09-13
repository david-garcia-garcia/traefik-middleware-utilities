package reclaim

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// assertCanceledBind fails unless Open returned ctx.Err() and no pointer.
func assertCanceledBind(t *testing.T, value any, err error, ctx context.Context) {
	t.Helper()
	if !errors.Is(err, ctx.Err()) {
		t.Fatalf("Open err %v, want %v", err, ctx.Err())
	}
	if value != nil {
		t.Fatalf("Open returned pointer %p, want nil", value)
	}
}

func TestTable_OpenCanceledDuringCreateReturnsCtxErr(t *testing.T) {
	const rounds = 50
	h := &recHandler{}
	for round := 0; round < rounds; round++ {
		tab := New(Config{Grace: 0})
		ctx, cancel := context.WithCancel(context.Background())
		item := &counterClose{}
		started := make(chan struct{})
		go func() { <-started; cancel() }()
		value, err := tab.Open(ctx, "k", recLogger(h), func() (any, error) {
			close(started)
			<-ctx.Done()
			return item, nil
		}, Hooks{Close: item.Close})
		assertCanceledBind(t, value, err, ctx)
		tab.Reset()
	}
}

func TestTable_OpenAlreadyDoneAwakeBindReturnsCtxErr(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: 0})
	item := &counterClose{}
	live, cancelLive := context.WithCancel(context.Background())
	defer cancelLive()
	stored, err := tab.Open(live, "k", recLogger(h), func() (any, error) { return item, nil }, Hooks{Close: item.Close})
	if err != nil {
		t.Fatalf("live Open: %v", err)
	}

	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	value, err := tab.Open(done, "k", recLogger(h), func() (any, error) {
		t.Fatal("create ran on awake bind")
		return nil, nil
	}, Hooks{})
	assertCanceledBind(t, value, err, done)
	if item.closes.Load() != 0 {
		t.Fatal("Close ran while a live holder still had the value")
	}

	again, err := tab.Open(live, "k", recLogger(h), func() (any, error) {
		t.Fatal("create ran again")
		return nil, nil
	}, Hooks{})
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	if again != stored {
		t.Fatal("live holder lost the incarnation")
	}
}

func TestTable_OpenAlreadyDoneReclaimReturnsCtxErr(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	life := &lifecycle{}
	first, cancelFirst := context.WithCancel(context.Background())
	stored, err := tab.Open(first, "k", recLogger(h), func() (any, error) { return life, nil }, Hooks{
		Sleep: life.Sleep,
		Wake:  life.Wake,
		Close: life.Close,
	})
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	cancelFirst()
	waitKeyMsg(t, h, MsgOrphan, "k")

	done, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	value, err := tab.Open(done, "k", recLogger(h), func() (any, error) {
		t.Fatal("create ran on reclaim")
		return nil, nil
	}, Hooks{})
	assertCanceledBind(t, value, err, done)
	if life.count("wake") != 1 {
		t.Fatalf("wake ran %d times, want 1", life.count("wake"))
	}
	if life.count("close") != 0 {
		t.Fatal("Close ran during grace after a canceled reclaim")
	}
	if stored != life {
		t.Fatal("stored value was not the lifecycle instrument")
	}
}

func TestTable_OpenCanceledCreatorWaiterKeepsLiveValue(t *testing.T) {
	h := &recHandler{}
	tab := New(Config{Grace: graceNoRace})
	item := &counterClose{}
	creating := make(chan struct{})
	waiterStarted := make(chan struct{})

	creator, cancelCreator := context.WithCancel(context.Background())
	waiter, cancelWaiter := context.WithCancel(context.Background())
	defer cancelWaiter()

	var waiterValue any
	var waiterErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-creating
		close(waiterStarted)
		waiterValue, waiterErr = tab.Open(waiter, "k", recLogger(h), func() (any, error) {
			t.Error("waiter ran create")
			return nil, nil
		}, Hooks{})
	}()

	creatorValue, creatorErr := tab.Open(creator, "k", recLogger(h), func() (any, error) {
		close(creating)
		<-waiterStarted
		time.Sleep(10 * time.Millisecond)
		cancelCreator()
		<-creator.Done()
		return item, nil
	}, Hooks{Close: item.Close})
	wg.Wait()

	assertCanceledBind(t, creatorValue, creatorErr, creator)
	if waiterErr != nil {
		t.Fatalf("waiter Open: %v", waiterErr)
	}
	if waiterValue != item {
		t.Fatalf("waiter got %p, want the created value", waiterValue)
	}
	if item.closes.Load() != 0 {
		t.Fatal("Close ran while the waiter still held the value")
	}
	tab.Reset()
}
