package tokenbucket

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRepro_EvalWaitNaNFailOpen requires Allow to return errEvalWait when Eval wait is not a finite number.
func TestRepro_EvalWaitNaNFailOpen(t *testing.T) {
	cases := []struct {
		name string
		wait string
	}{
		{name: "nan", wait: "nan"},
		{name: "+Inf", wait: "+Inf"},
		{name: "-Inf", wait: "-Inf"},
		{name: "inf", wait: "inf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, addr := startTestFakeRedis(t)
			fake.setEvalReply(arrayBulks("true", tc.wait, "0"))
			client := newSimpleRedisForTest(t, addr)
			limiter, err := NewRedis(client, 1, 1, time.Second, testTTL)
			if err != nil {
				t.Fatal(err)
			}
			status, allowErr := limiter.Allow(context.Background(), "k")
			if status.Allowed && allowErr == nil {
				t.Fatalf("fail-open: Allow wait %q returned an allowed status %+v with a nil error", tc.wait, status)
			}
			if !errors.Is(allowErr, errEvalWait) {
				t.Fatalf("wait %q: want errEvalWait, got %v", tc.wait, allowErr)
			}
			if status != (Status{}) {
				t.Fatalf("wait %q: want the zero Status, got %+v", tc.wait, status)
			}
		})
	}
}
