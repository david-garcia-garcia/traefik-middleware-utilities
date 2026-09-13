package simpleredis

import (
	"context"
	"testing"
)

// TestWriteCommandDoesNotInjectCommands locks that keys and values containing
// CRLF and an inline PING payload round-trip as data on a strict-RESP fake.
func TestWriteCommandDoesNotInjectCommands(t *testing.T) {
	injected := "\r\n*1\r\n$4\r\nPING\r\n"
	key := "range" + injected
	value := "payload" + injected
	fake, addr := startFakeRedis(t, map[string]string{})
	client := newTestRedis(t, Config{Host: addr, MaxRetries: -1})

	if err := client.Set(context.Background(), key, []byte(value), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := client.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != value {
		t.Fatalf("Get = %q, want %q", got, value)
	}
	setArgv := fake.lastSetCommand()
	if len(setArgv) < 3 || setArgv[1] != key || setArgv[2] != value {
		t.Fatalf("SET argv = %q, want key and value as single bulk args", setArgv)
	}
	for _, arg := range setArgv {
		if arg == "PING" {
			t.Fatalf("fake parsed injected PING as a command in %q", setArgv)
		}
	}
}
