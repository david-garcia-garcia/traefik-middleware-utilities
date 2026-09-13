# Nitpicks

1. [hard] Symmetry and consistency — `simpleredis/pool_test.go:237` — the four new handshake no-redial tests name the `New` result `client`; every other test in this file (including sibling `TestHandshakeAuthRejectedMapsToNoAuthAndIsNotPooled` and `TestHandshakeSelectRejectedAfterAuthIsNotPooled`) names that same SimpleRedis role `redis`

```go
func TestHandshakeAuthEOFMustNotOpenSecondConnection(t *testing.T) {
	accepts, addr := startAcceptFake(t, func(_ net.Conn, reader *bufio.Reader) {
		if _, err := readCommand(reader); err != nil {
			return
		}
		// close with no AUTH reply
	})
	client := New(Config{Host: addr, Pass: "secret", MaxRetries: 1, MinRetryBackoff: -1})
	_, err := client.Get(context.Background(), "k")
	if err == nil {
		t.Fatal("want error after AUTH EOF")
	}
	if atomic.LoadInt32(accepts) != 1 {
		t.Fatalf("TCP accepts = %d, want 1; err=%v", atomic.LoadInt32(accepts), err)
	}
}
```

Same `client := New(...)` in `TestHandshakeSelectEOFMustNotOpenSecondConnection`, `TestHandshakeAuthLoadingMustNotOpenSecondConnection`, and `TestHandshakeAuthMaxClientsMustNotOpenSecondConnection`.
   → Rename `client` to `redis` in those four tests
   Status: done
   Argument: renamed `client` to `redis` in the four handshake no-redial tests (`pool_test.go`).
