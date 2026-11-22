package tokenbucket

import (
	"errors"
	"math"
	"testing"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestRepro_InfRateRejected asserts NewMemory and NewRedis reject +Inf and -Inf.
func TestRepro_InfRateRejected(t *testing.T) {
	t.Run("NewMemory/+Inf", func(t *testing.T) {
		limiter, err := NewMemory(math.Inf(1), 1, 0, testTTL)
		if err == nil {
			t.Errorf("NewMemory(+Inf) returned limiter %v, want errRate", limiter)
			return
		}
		if !errors.Is(err, errRate) {
			t.Fatalf("NewMemory(+Inf) err %v, want errRate", err)
		}
		if limiter != nil {
			t.Fatalf("NewMemory(+Inf) limiter %v with err %v", limiter, err)
		}
	})
	t.Run("NewRedis/+Inf", func(t *testing.T) {
		client, err := simpleredis.New(simpleredis.Config{Host: "127.0.0.1:1"})
		if err != nil {
			t.Fatal(err)
		}
		limiter, err := NewRedis(client, math.Inf(1), 1, 0, testTTL)
		if err == nil {
			t.Errorf("NewRedis(+Inf) returned limiter %v, want errRate", limiter)
			return
		}
		if !errors.Is(err, errRate) {
			t.Fatalf("NewRedis(+Inf) err %v, want errRate", err)
		}
		if limiter != nil {
			t.Fatalf("NewRedis(+Inf) limiter %v with err %v", limiter, err)
		}
	})
	t.Run("NewMemory/-Inf", func(t *testing.T) {
		limiter, err := NewMemory(math.Inf(-1), 1, 0, testTTL)
		if err == nil {
			t.Errorf("NewMemory(-Inf) returned limiter %v, want errRate", limiter)
			return
		}
		if !errors.Is(err, errRate) {
			t.Fatalf("NewMemory(-Inf) err %v, want errRate", err)
		}
		if limiter != nil {
			t.Fatalf("NewMemory(-Inf) limiter %v with err %v", limiter, err)
		}
	})
}
