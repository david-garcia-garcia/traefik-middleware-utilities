package simpleredis

import (
	"context"
	"errors"
	"testing"
)

// TestTraefik12078_PasswordWithTrailingNewline: traefik/traefik#12078.
// The report: the Redis rate limiter always fails with WRONGPASS (and once EOF) although the password is
// right. A collaborator found the Kubernetes Secret values ended in "\n" (base64 "Cg==" is "\n", not "").
// A Secret read into Config.Pass reaches AUTH byte for byte, so the same mistake here must fail the same
// way: the configured password authenticates, the same password plus "\n" is rejected as redis:noauth.
// The CI requirepass engines use the password "secret".
func TestTraefik12078_PasswordWithTrailingNewline(t *testing.T) {
	runForEachLiveEngine(t, "SIMPLEREDIS_LIVE_REDIS_AUTH", "SIMPLEREDIS_LIVE_DRAGONFLY_AUTH", func(t *testing.T, addr string) {
		t.Helper()
		cases := []struct {
			name     string
			pass     string
			wantAuth bool
		}{
			{name: "control_exact_password", pass: "secret", wantAuth: true},
			{name: "secret_with_trailing_newline", pass: "secret\n", wantAuth: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				client := newTestRedis(t, Config{Host: addr, Pass: tc.pass})
				t.Cleanup(client.Close)
				_, err := client.Get(context.Background(), "traefik-12078-probe")
				t.Logf("Pass=%q: Get err=%v", tc.pass, err)
				if tc.wantAuth {
					if err != nil && !errors.Is(err, ErrMiss) {
						t.Fatalf("exact password: Get = %v, want success or %s", err, RedisMiss)
					}
					return
				}
				if err == nil || err.Error() != RedisNoAuth {
					t.Fatalf("password with trailing newline: Get = %v, want %s", err, RedisNoAuth)
				}
			})
		}
	})
}
