package simpleredis

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// TestTLSRoundTripOwnCA proves Set then Get over TLS with an own CA and an empty ServerName: crypto/tls takes the
// name from the localhost host, and the fake finished the handshake before it read the first RESP command.
func TestTLSRoundTripOwnCA(t *testing.T) {
	fixture := testTLSFixture(t)
	fake, state, addr := startTLSFakeRedis(t, map[string]string{}, tlsFakeOptionsForTest{})
	redis := newTestRedis(t, Config{Host: localhostAddrForTest(t, addr), TLS: fixture.clientTLSConfig(t, false)})
	t.Cleanup(redis.Close)

	if err := redis.Set(context.Background(), "k", []byte("v"), 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := redis.Get(context.Background(), "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("Get = %q, %v, want v", got, err)
	}
	handshakes, _, _ := state.snapshot()
	if handshakes != 1 || fake.connections() != 1 {
		t.Fatalf("handshakes=%d connections=%d, want 1, 1", handshakes, fake.connections())
	}
}

// TestTLSClientCertificateForMutualTLS proves the fake that requires a client certificate sees the one in Config.TLS.
func TestTLSClientCertificateForMutualTLS(t *testing.T) {
	fixture := testTLSFixture(t)
	_, state, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, tlsFakeOptionsForTest{requireClientCert: true})
	redis := newTestRedis(t, Config{Host: addr, TLS: fixture.clientTLSConfig(t, true)})
	t.Cleanup(redis.Close)

	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, clientCerts, _ := state.snapshot(); clientCerts != 1 {
		t.Fatalf("client certificates seen = %d, want 1", clientCerts)
	}
}

// TestTLS12OnlyServer proves a client with no version bounds talks to a TLS 1.2-only server.
func TestTLS12OnlyServer(t *testing.T) {
	fixture := testTLSFixture(t)
	_, state, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, tlsFakeOptionsForTest{maxTLS12: true})
	redis := newTestRedis(t, Config{Host: addr, TLS: &tls.Config{RootCAs: fixture.caPool}}) //nolint:gosec // G402: MinVersion left unset on purpose
	t.Cleanup(redis.Close)

	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, _, versions := state.snapshot(); len(versions) != 1 || versions[0] != tls.VersionTLS12 {
		t.Fatalf("negotiated versions = %x, want [%x]", versions, tls.VersionTLS12)
	}
}

// TestNilTLSStaysPlainTCP proves a nil Config.TLS sends RESP as the first bytes on the socket.
func TestNilTLSStaysPlainTCP(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	redis := newTestRedis(t, Config{Host: addr})
	t.Cleanup(redis.Close)
	if redis.tlsConfig != nil {
		t.Fatal("tlsConfig set for a nil Config.TLS")
	}
	// The plain fake parses RESP from the first byte; a ClientHello would never reach GET.
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, _, gets := fake.handshakeCounts(); gets != 1 {
		t.Fatalf("GET = %d, want 1", gets)
	}
}

// TestTLSConfigFrozenAtNew proves New keeps a clone: writes to the caller's tls.Config after New do not reach the client.
func TestTLSConfigFrozenAtNew(t *testing.T) {
	fixture := testTLSFixture(t)
	_, _, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, tlsFakeOptionsForTest{})
	callerTLS := &tls.Config{RootCAs: fixture.caPool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	redis := newTestRedis(t, Config{Host: addr, TLS: callerTLS})
	t.Cleanup(redis.Close)

	callerTLS.ServerName = "wrong.example"
	callerTLS.RootCAs = x509.NewCertPool()
	if _, err := redis.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("Get after caller writes = %v, want success", err)
	}
	if redis.tlsConfig == callerTLS {
		t.Fatal("client shares the caller's *tls.Config")
	}
}

// TestTLSHandshakeCountsAgainstDialTimeout proves a peer that never answers the ClientHello ends the attempt at
// DialTimeout, not at CommandTimeout, and that it is redis:unreachable.
func TestTLSHandshakeCountsAgainstDialTimeout(t *testing.T) {
	fixture := testTLSFixture(t)
	acceptCount, addr := startAcceptFake(t, func(conn net.Conn, _ *bufio.Reader) { drainUntilPeerCloseForTest(conn) })
	redis := newTestRedis(t, Config{Host: addr, TLS: fixture.clientTLSConfig(t, false),
		DialTimeout: 200 * time.Millisecond, CommandTimeout: 2 * time.Second, MaxRetries: -1})
	t.Cleanup(redis.Close)

	started := time.Now()
	_, err := redis.Get(context.Background(), "k")
	elapsed := time.Since(started)
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if elapsed < 150*time.Millisecond || elapsed > time.Second {
		t.Fatalf("attempt took %v, want about DialTimeout (200ms)", elapsed)
	}
	if got := atomic.LoadInt32(acceptCount); got != 1 {
		t.Fatalf("accepts = %d, want 1", got)
	}
}

// TestTLSRejectionReturnsRedisTLS covers every A TLS rejection returns redis:tls scenario on the in-process fake.
func TestTLSRejectionReturnsRedisTLS(t *testing.T) {
	fixture := testTLSFixture(t)
	rows := []struct {
		name string
		opts tlsFakeOptionsForTest
		// config builds the client TLS configuration for that row.
		config   func() *tls.Config
		pass     string
		hostIsIP bool
	}{
		{"unknownCA", tlsFakeOptionsForTest{}, func() *tls.Config {
			return &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}
		}, "", true},
		{"serverNameMismatch", tlsFakeOptionsForTest{}, func() *tls.Config {
			config := fixture.clientTLSConfig(t, false)
			config.ServerName = "wrong.example"
			return config
		}, "", true},
		{"ipHostWithoutIPSAN", tlsFakeOptionsForTest{noIPSAN: true}, func() *tls.Config {
			return fixture.clientTLSConfig(t, false)
		}, "", true},
		{"clientCertRefusedTLS12Handshake", tlsFakeOptionsForTest{requireClientCert: true, maxTLS12: true}, func() *tls.Config {
			return fixture.clientTLSConfig(t, false)
		}, "", true},
		{"clientCertRefusedTLS13FirstRead", tlsFakeOptionsForTest{requireClientCert: true}, func() *tls.Config {
			return fixture.clientTLSConfig(t, false)
		}, "", true},
		{"clientCertRefusedTLS13DuringAuth", tlsFakeOptionsForTest{requireClientCert: true}, func() *tls.Config {
			return fixture.clientTLSConfig(t, false)
		}, "secret", true},
	}
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			fake, _, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, row.opts)
			redis := newTestRedis(t, Config{Host: addr, TLS: row.config(), Pass: row.pass, MaxRetries: 1, MinRetryBackoff: -1})
			t.Cleanup(redis.Close)
			_, err := redis.Get(context.Background(), "hit")
			requireRedisTLSNotRetriedForTest(t, redis, err, fake.connections())
		})
	}
}

// TestTLSPeerAnswersClientHelloWithPlainText proves a plain peer that writes a RESP error to the ClientHello is redis:tls.
func TestTLSPeerAnswersClientHelloWithPlainText(t *testing.T) {
	fixture := testTLSFixture(t)
	// A plain peer answers the first bytes it reads (the ClientHello) with a RESP error line, then drains.
	_, addr := startAcceptFake(t, func(conn net.Conn, reader *bufio.Reader) {
		clientHello := make([]byte, 512)
		if _, err := reader.Read(clientHello); err != nil {
			return
		}
		_, _ = io.WriteString(conn, "-ERR unknown command\r\n")
		drainUntilPeerCloseForTest(conn)
	})
	redis := newTestRedis(t, Config{Host: addr, TLS: fixture.clientTLSConfig(t, false), MaxRetries: -1})
	t.Cleanup(redis.Close)
	_, err := redis.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisTLS {
		t.Fatalf("Get = %v, want %s", err, RedisTLS)
	}
}

// TestTLSRejectionWithRetriesIsOneConnection proves MaxRetries 3 does not redial an untrusted server.
func TestTLSRejectionWithRetriesIsOneConnection(t *testing.T) {
	fake, _, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, tlsFakeOptionsForTest{})
	redis := newTestRedis(t, Config{Host: addr, TLS: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
		MaxRetries: 3, MinRetryBackoff: -1})
	t.Cleanup(redis.Close)
	_, err := redis.Get(context.Background(), "hit")
	requireRedisTLSNotRetriedForTest(t, redis, err, fake.connections())
}

// TestTLSSilentPlainPeerStaysUnreachable proves a TLS client against a peer that never writes is redis:unreachable
// and is retried like a plain dial failure.
func TestTLSSilentPlainPeerStaysUnreachable(t *testing.T) {
	fixture := testTLSFixture(t)
	acceptCount, addr := startAcceptFake(t, func(conn net.Conn, _ *bufio.Reader) { drainUntilPeerCloseForTest(conn) })
	redis := newTestRedis(t, Config{Host: addr, TLS: fixture.clientTLSConfig(t, false),
		MaxRetries: 1, MinRetryBackoff: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(redis.Close)
	_, err := redis.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
	if !IsUnreachable(err) {
		t.Fatalf("IsUnreachable(%v) = false", err)
	}
	if got := atomic.LoadInt32(acceptCount); got != 2 {
		t.Fatalf("accepts = %d, want 2", got)
	}
}

// TestTLSHostRefusesTCP proves a refused TCP connect with TLS set is redis:unreachable.
func TestTLSHostRefusesTCP(t *testing.T) {
	fixture := testTLSFixture(t)
	redis := newTestRedis(t, Config{Host: "127.0.0.1:1", TLS: fixture.clientTLSConfig(t, false), MaxRetries: -1})
	t.Cleanup(redis.Close)
	_, err := redis.Get(context.Background(), "k")
	if err == nil || err.Error() != RedisUnreachable {
		t.Fatalf("Get = %v, want %s", err, RedisUnreachable)
	}
}

// TestTLSCallerCancelDuringHandshake proves a caller cancel while the handshake waits surfaces as context.Canceled.
func TestTLSCallerCancelDuringHandshake(t *testing.T) {
	fixture := testTLSFixture(t)
	addr := startStallRedis(t)
	redis := newTestRedis(t, Config{Host: addr, TLS: fixture.clientTLSConfig(t, false), DialTimeout: 2 * time.Second, CommandTimeout: 3 * time.Second})
	t.Cleanup(redis.Close)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := redis.Get(ctx, "k")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get = %v, want context.Canceled", err)
	}
}

// requireRedisTLSNotRetriedForTest asserts err is exactly the redis:tls sentinel, not unreachable, on one TCP
// connection, with nothing left in the idle pool.
func requireRedisTLSNotRetriedForTest(t *testing.T, redis *SimpleRedis, err error, connections int) {
	t.Helper()
	if err == nil || err.Error() != RedisTLS {
		t.Fatalf("Get = %v, want %s", err, RedisTLS)
	}
	if !errors.Is(err, ErrTLS) {
		t.Fatalf("errors.Is(%v, ErrTLS) = false", err)
	}
	if IsUnreachable(err) {
		t.Fatalf("IsUnreachable(%v) = true, want false", err)
	}
	if connections != 1 {
		t.Fatalf("TCP connections = %d, want 1", connections)
	}
	if pooledIdle(redis) != 0 {
		t.Fatalf("idle = %d, want 0", pooledIdle(redis))
	}
}

// localhostAddrForTest rewrites a 127.0.0.1:<port> listener address to localhost:<port>.
func localhostAddrForTest(t *testing.T, addr string) string {
	t.Helper()
	const loopbackPrefix = "127.0.0.1:"
	if len(addr) <= len(loopbackPrefix) || addr[:len(loopbackPrefix)] != loopbackPrefix {
		t.Fatalf("listener address %q is not 127.0.0.1:<port>", addr)
	}
	return "localhost:" + addr[len(loopbackPrefix):]
}
