package simpleredis

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// tlsFakeOptionsForTest shapes the in-process TLS fake.
type tlsFakeOptionsForTest struct {
	// requireClientCert makes the fake require and verify a client certificate signed by the fixture CA.
	requireClientCert bool
	// maxTLS12 caps the fake at TLS 1.2.
	maxTLS12 bool
	// noIPSAN serves the certificate without the 127.0.0.1 IP SAN.
	noIPSAN bool
}

// tlsFakeStateForTest is what the TLS fake observed about completed handshakes.
type tlsFakeStateForTest struct {
	mu sync.Mutex
	// handshakes counts server handshakes that completed before any RESP was read.
	handshakes int
	// clientCerts counts completed handshakes where the client presented a certificate.
	clientCerts int
	// versions holds the negotiated TLS version of each completed handshake.
	versions []uint16
}

// snapshot returns the handshake count, client-certificate count, and negotiated versions so far.
func (state *tlsFakeStateForTest) snapshot() (handshakes, clientCerts int, versions []uint16) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.handshakes, state.clientCerts, append([]uint16(nil), state.versions...)
}

// startTLSFakeRedis serves the same in-process RESP fake as startFakeRedis behind tls.NewListener.
// The server handshake runs before RESP is served. A failed handshake keeps the socket open and drains what the
// client sent until the client closes, so the TLS alert reaches the client instead of a TCP reset that would
// discard it (the TLS 1.3 client-certificate alert arrives after the client already wrote its first command).
func startTLSFakeRedis(tb testing.TB, store map[string]string, opts tlsFakeOptionsForTest) (server *fakeRedis, state *tlsFakeStateForTest, listenAddr string) {
	tb.Helper()
	fixture := testTLSFixture(tb)
	certPEM, keyPEM := fixture.serverPEM, fixture.serverKeyPEM
	if opts.noIPSAN {
		certPEM, keyPEM = fixture.serverNoIPPEM, fixture.serverNoIPKeyPEM
	}
	serverPair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		tb.Fatal(err)
	}
	serverConfig := &tls.Config{Certificates: []tls.Certificate{serverPair}, MinVersion: tls.VersionTLS12}
	if opts.requireClientCert {
		serverConfig.ClientAuth = tls.RequireAndVerifyClientCert
		serverConfig.ClientCAs = fixture.caPool
	}
	if opts.maxTLS12 {
		serverConfig.MaxVersion = tls.VersionTLS12
	}

	rawListener := listenLocalTCPForTest(tb)
	fake := newFakeRedisForTest(store)
	state = &tlsFakeStateForTest{}
	fake.admitConn = func(conn net.Conn) bool {
		return admitTLSConnForTest(conn, state)
	}
	go fake.acceptLoop(tls.NewListener(rawListener, serverConfig))
	return fake, state, rawListener.Addr().String()
}

// admitTLSConnForTest completes the server handshake on conn and records it, or drains the refused socket.
func admitTLSConnForTest(conn net.Conn, state *tlsFakeStateForTest) bool {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return false
	}
	// Bound the handshake so a client that never finishes does not pin the goroutine.
	_ = tlsConn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsConn.Handshake(); err != nil {
		drainUntilPeerCloseForTest(tlsConn.NetConn())
		return false
	}
	_ = tlsConn.SetDeadline(time.Time{})
	connState := tlsConn.ConnectionState()
	state.mu.Lock()
	state.handshakes++
	if len(connState.PeerCertificates) > 0 {
		state.clientCerts++
	}
	state.versions = append(state.versions, connState.Version)
	state.mu.Unlock()
	return true
}

// drainUntilPeerCloseForTest reads and discards conn until the peer closes or 5 seconds pass.
func drainUntilPeerCloseForTest(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.Copy(io.Discard, conn)
}
