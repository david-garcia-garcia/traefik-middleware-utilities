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
			allowed, wait, allowErr := limiter.Allow(context.Background(), "k")
			if allowed && allowErr == nil {
				t.Fatalf("fail-open: Allow wait %q returned (true, %v, nil)", tc.wait, wait)
			}
			if allowErr == nil {
				t.Fatalf("want error for wait %q, got allowed=%v wait=%v err=nil", tc.wait, allowed, wait)
			}
			if !errors.Is(allowErr, errEvalWait) {
				t.Fatalf("wait %q: want errEvalWait, got %v", tc.wait, allowErr)
			}
			if wait != 0 {
				t.Fatalf("wait %q: want wait 0, got %v", tc.wait, wait)
			}
			if allowed {
				t.Fatalf("wait %q: want allowed false, got true", tc.wait)
			}
		})
	}
}
