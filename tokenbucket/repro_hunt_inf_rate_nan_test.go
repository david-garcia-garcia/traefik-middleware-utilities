package tokenbucket

import (
	"errors"
	"math"
	"testing"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TestRepro_InfRateElapsedZeroNaNFailOpen asserts New rejects +Inf and -Inf.
// Dest used to accept +Inf; the second Allow at elapsed=0 then did Inf*0=NaN and fail-opened.
func TestRepro_InfRateElapsedZeroNaNFailOpen(t *testing.T) {
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
		client := simpleredis.New(simpleredis.Config{Host: "127.0.0.1:1"})
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
