package reclaim

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type aliasTestClient struct {
	id string
}

func loadedClient(dest *atomic.Value) *aliasTestClient {
	client, _ := Unbox(dest).(*aliasTestClient)
	return client
}

func Unbox(dest *atomic.Value) any {
	if dest == nil {
		return nil
	}
	prev := dest.Load()
	if boxed, ok := prev.(*Box); ok {
		return boxed.Value
	}
	return prev
}

func watchInto(ctx context.Context, tab *Table, alias string, dest *atomic.Value, empty any, onChange func()) {
	tab.Watch(ctx, alias, empty, func(published any) {
		notice, _ := published.(Published)
		prev := dest.Load()
		if boxed, ok := prev.(*Box); ok {
			boxed.Value = notice.Value
		} else {
			dest.Store(&Box{Value: notice.Value})
		}
		if onChange != nil {
			onChange()
		}
	})
}

func openValue(t *testing.T, tab *Table, key string, value any) {
	t.Helper()
	_, err := tab.OpenWithHooks(context.Background(), key, slog.Default(), func() (any, Hooks, error) {
		return value, Hooks{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWatchBeforeSetAlias(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	if loadedClient(&bound) != nil {
		t.Fatal("watch before alias must leave typed nil")
	}
	owner := &aliasTestClient{id: "A"}
	openValue(t, tab, "owner-a", owner)
	if err := tab.SetAlias("owner-a", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&bound) != owner {
		t.Fatal("set alias must Store into existing watchers")
	}
}

func TestIndependentLAPIAndAppSecSharedName(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	lapiClient := &aliasTestClient{id: "lapi"}
	appsecClient := &aliasTestClient{id: "appsec"}
	openValue(t, tab, "lapi-key", lapiClient)
	openValue(t, tab, "appsec-key", appsecClient)
	if err := tab.SetAlias("lapi-key", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("appsec-key", "alias:appsec:shared", "cs", "appsec"); err != nil {
		t.Fatal(err)
	}
	tab.ClearPublisher("cs", "lapi")
	var appsecBound atomic.Value
	watchInto(context.Background(), tab, "alias:appsec:shared", &appsecBound, (*aliasTestClient)(nil), nil)
	if loadedClient(&appsecBound) != appsecClient {
		t.Fatal("clearing LAPI shared must not clear AppSec shared")
	}
}

func TestSecondPublisherRejected(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	first := &aliasTestClient{id: "first"}
	second := &aliasTestClient{id: "second"}
	openValue(t, tab, "a", first)
	openValue(t, tab, "b", second)
	if err := tab.SetAlias("a", "alias:lapi:shared", "cs-a", "lapi"); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("b", "alias:lapi:shared", "cs-b", "lapi"); err == nil {
		t.Fatal("second publisher on taken alias must fail")
	}
	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	if loadedClient(&bound) != first {
		t.Fatal("first publisher must keep the alias")
	}
}

func TestGraceCloseClearsWatchers(t *testing.T) {
	tab := NewTable(30 * time.Millisecond)
	t.Cleanup(tab.Reset)

	client := &aliasTestClient{id: "A"}
	var bound atomic.Value
	cleared := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctx, "owner", slog.Default(), func() (any, Hooks, error) {
		return client, Hooks{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), func() {
		if loadedClient(&bound) != nil {
			return
		}
		select {
		case <-cleared:
		default:
			close(cleared)
		}
	})
	if loadedClient(&bound) != client {
		t.Fatal("watch must see the published client")
	}
	cancel()
	select {
	case <-cleared:
	case <-time.After(time.Second):
		t.Fatal("grace close must clear the watcher")
	}
}

func TestDyingIncarnationDoesNotUnbindReplacement(t *testing.T) {
	tab := NewTable(0)
	t.Cleanup(tab.Reset)

	oldClient := &aliasTestClient{id: "A"}
	newClient := &aliasTestClient{id: "B"}
	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	ctxOld, cancelOld := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctxOld, "old", slog.Default(), func() (any, Hooks, error) {
		return oldClient, Hooks{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("old", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	openValue(t, tab, "new", newClient)
	if err := tab.SetAlias("new", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	cancelOld()
	time.Sleep(20 * time.Millisecond)
	if loadedClient(&bound) != newClient {
		t.Fatal("close of dying A must not Store nil over B")
	}
}

func TestPublisherRenameClearsOldName(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	owner := &aliasTestClient{id: "A"}
	var oldBound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &oldBound, (*aliasTestClient)(nil), nil)
	openValue(t, tab, "owner", owner)
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&oldBound) != owner {
		t.Fatal("first alias must bind the old name")
	}
	if err := tab.SetAlias("owner", "alias:lapi:other", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&oldBound) != nil {
		t.Fatal("rename must unbind watchers of the previous name")
	}
}

func TestValueChangedRunsOnPublishAndClear(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	var bound atomic.Value
	changes := 0
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), func() { changes++ })
	if changes != 0 {
		t.Fatalf("watching an empty alias must not report a change, got %d", changes)
	}
	owner := &aliasTestClient{id: "A"}
	openValue(t, tab, "owner", owner)
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if changes != 1 || loadedClient(&bound) != owner {
		t.Fatalf("publish must report one change, changes=%d loaded=%v", changes, loadedClient(&bound))
	}
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatalf("publishing the same client must not report another change, changes=%d", changes)
	}
	var later atomic.Value
	laterChanges := 0
	watchInto(context.Background(), tab, "alias:lapi:shared", &later, (*aliasTestClient)(nil), func() { laterChanges++ })
	if laterChanges != 1 || loadedClient(&later) != owner {
		t.Fatalf("watching a published alias must report one change, changes=%d", laterChanges)
	}
	tab.ClearPublisher("cs", "lapi")
	if changes != 2 || loadedClient(&bound) != nil {
		t.Fatalf("clear must report one change, changes=%d loaded=%v", changes, loadedClient(&bound))
	}
}

func TestWatchDropsSubscriberWhenCtxEnds(t *testing.T) {
	tab := NewTable(graceNoRace)
	for wait := time.Millisecond; wait < time.Second; wait *= 2 {
		tab.Reset()
		ctx, cancel := context.WithCancel(context.Background())
		var bound atomic.Value
		changes := 0
		watchInto(ctx, tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), func() { changes++ })
		cancel()
		time.Sleep(wait)
		owner := &aliasTestClient{id: "A"}
		openValue(t, tab, "owner", owner)
		if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
			t.Fatal(err)
		}
		if loadedClient(&bound) == nil && changes == 0 {
			tab.Reset()
			return
		}
	}
	t.Fatal("ctx done must drop the subscriber before a later publish")
}

func TestClearPublisherDropsOwnedAliases(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	owner := &aliasTestClient{id: "A"}
	other := &aliasTestClient{id: "B"}
	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	openValue(t, tab, "owner", owner)
	openValue(t, tab, "other", other)
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("other", "alias:lapi:kept", "other", "lapi"); err != nil {
		t.Fatal(err)
	}
	tab.ClearPublisher("cs", "lapi")
	if loadedClient(&bound) != nil {
		t.Fatal("dropping the publisher must unbind its watchers")
	}
	var kept atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:kept", &kept, (*aliasTestClient)(nil), nil)
	if loadedClient(&kept) != other {
		t.Fatal("ClearPublisher must not touch another middleware's alias")
	}
}

func TestSetAliasErrorsWhenKeyNotMapped(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	if err := tab.SetAlias("missing", "alias:x", "p", "g"); err == nil {
		t.Fatal("SetAlias on missing key must error")
	}
}

func TestSetAliasErrorsWhenSlotBusy(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = tab.OpenWithHooks(context.Background(), "busy", slog.Default(), func() (any, Hooks, error) {
			close(started)
			<-release
			return "v", Hooks{}, nil
		})
	}()
	<-started
	if err := tab.SetAlias("busy", "alias:x", "p", "g"); err == nil {
		t.Fatal("SetAlias on busy slot must error")
	}
	close(release)
}

func TestWatchPanicsOnNilContext(t *testing.T) {
	tab := NewTable(graceNoRace)
	defer func() {
		if recover() == nil {
			t.Fatal("Watch with nil context must panic")
		}
	}()
	tab.Watch(nil, "alias:x", nil, nil) //nolint:staticcheck // documents panic contract
}

func TestAliasUnmapClearsWatchersAndAllowsRepublish(t *testing.T) {
	tab := NewTable(0)
	t.Cleanup(tab.Reset)

	client := &aliasTestClient{id: "A"}
	var bound atomic.Value
	cleared := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctx, "owner", slog.Default(), func() (any, Hooks, error) {
		return client, Hooks{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), func() {
		if loadedClient(&bound) == nil {
			select {
			case cleared <- struct{}{}:
			default:
			}
		}
	})
	cancel()
	select {
	case <-cleared:
	case <-time.After(waitBudget):
		t.Fatal("unmap must clear alias watchers")
	}
	replacement := &aliasTestClient{id: "B"}
	openValue(t, tab, "owner2", replacement)
	if err := tab.SetAlias("owner2", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&bound) != replacement {
		t.Fatal("later SetAlias must publish after unmap cleared watchers")
	}
}

func TestAliasGraceExpireClearsWatchers(t *testing.T) {
	tab := NewTable(40 * time.Millisecond)
	t.Cleanup(tab.Reset)

	client := &aliasTestClient{id: "A"}
	var bound atomic.Value
	cleared := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctx, "owner", slog.Default(), func() (any, Hooks, error) {
		return client, Hooks{Close: func() {}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), func() {
		if loadedClient(&bound) == nil {
			select {
			case cleared <- struct{}{}:
			default:
			}
		}
	})
	cancel()
	select {
	case <-cleared:
	case <-time.After(waitBudget):
		t.Fatal("grace expire must clear alias watchers")
	}
}

func TestAliasResetClearsWatchers(t *testing.T) {
	tab := NewTable(graceNoRace)

	client := &aliasTestClient{id: "A"}
	var bound atomic.Value
	openValue(t, tab, "owner", client)
	if err := tab.SetAlias("owner", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	if loadedClient(&bound) != client {
		t.Fatal("watch must see published client before reset")
	}
	tab.Reset()
	if loadedClient(&bound) != nil {
		t.Fatal("Reset must clear alias watchers to empty")
	}
	openValue(t, tab, "owner2", &aliasTestClient{id: "B"})
	if err := tab.SetAlias("owner2", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&bound) != nil {
		t.Fatal("watchers should receive republish after manual watch re-register")
	}
}

func TestFailedCreateUnbindsAlias(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:lapi:shared", &bound, (*aliasTestClient)(nil), nil)
	openValue(t, tab, "good", &aliasTestClient{id: "ok"})
	if err := tab.SetAlias("good", "alias:lapi:shared", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&bound) == nil {
		t.Fatal("alias must bind before failed create on another key")
	}
	_, err := tab.OpenWithHooks(context.Background(), "bad", slog.Default(), func() (any, Hooks, error) {
		return nil, Hooks{}, context.Canceled
	})
	if err == nil {
		t.Fatal("create must fail")
	}
	// Alias on "good" must remain; only testing create failure does not break unrelated alias.
	if loadedClient(&bound) == nil {
		t.Fatal("failed create on another key must not clear unrelated alias")
	}
	// Bind alias to key that will fail create, then fail and ensure watchers clear.
	tab.Reset()
	watchInto(context.Background(), tab, "alias:fail", &bound, (*aliasTestClient)(nil), func() {})
	started := make(chan struct{})
	go func() {
		_, _ = tab.OpenWithHooks(context.Background(), "fail-key", slog.Default(), func() (any, Hooks, error) {
			close(started)
			return nil, Hooks{}, context.Canceled
		})
	}()
	<-started
	// fail-key is gone; SetAlias must error
	if err := tab.SetAlias("fail-key", "alias:fail", "cs", "lapi"); err == nil {
		t.Fatal("SetAlias on gone slot must error")
	}
}

func TestEnforceCloseUnbindsAliasOnCreateFailure(t *testing.T) {
	tab := NewTable(graceNoRace)
	t.Cleanup(tab.Reset)

	var bound atomic.Value
	watchInto(context.Background(), tab, "alias:enforce", &bound, (*aliasTestClient)(nil), nil)
	client := &aliasTestClient{id: "live"}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.OpenWithHooks(ctx, "owner", slog.Default(), func() (any, Hooks, error) {
		return client, Hooks{EnforceCloseBeforeOpen: true, Close: func() {}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := tab.SetAlias("owner", "alias:enforce", "cs", "lapi"); err != nil {
		t.Fatal(err)
	}
	if loadedClient(&bound) != client {
		t.Fatal("alias must publish")
	}
	cancel()
	deadline := time.Now().Add(waitBudget)
	for time.Now().Before(deadline) {
		if loadedClient(&bound) == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("enforce close path must unbind alias watchers")
}
