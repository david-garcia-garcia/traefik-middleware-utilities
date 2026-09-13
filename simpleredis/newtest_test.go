package simpleredis

import "testing"

// newTestRedis is New for tests: fail the test when Config is rejected.
func newTestRedis(tb testing.TB, cfg Config) *SimpleRedis {
	tb.Helper()
	client, err := New(cfg)
	if err != nil {
		tb.Fatal(err)
	}
	return client
}
