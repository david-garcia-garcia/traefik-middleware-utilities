package simpleredis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// tlsFixtureForTest is the one certificate set the TLS fake, the Yaegi TLS probes, and the live TLS engines use:
// a P-256 CA, server certificates signed by it, and a client certificate signed by it. Keys are PKCS#8 because
// Dragonfly does not load SEC1 EC keys.
type tlsFixtureForTest struct {
	caPEM []byte
	// caPool trusts caPEM only.
	caPool *x509.CertPool
	// serverPEM / serverKeyPEM carry SANs localhost, redis-tls, dragonfly-tls and IP 127.0.0.1.
	serverPEM    []byte
	serverKeyPEM []byte
	// serverNoIPPEM / serverNoIPKeyPEM carry SAN localhost only, for the IP-host name mismatch.
	serverNoIPPEM    []byte
	serverNoIPKeyPEM []byte
	clientPEM        []byte
	clientKeyPEM     []byte
}

var (
	tlsFixtureOnce  sync.Once
	tlsFixtureValue *tlsFixtureForTest
	errTLSFixture   error
)

// tlsFixtureOutEnv names the directory TestWriteTLSFixtureFiles writes the PEM files into.
const tlsFixtureOutEnv = "SIMPLEREDIS_TLS_FIXTURE_OUT"

// testTLSFixture builds the certificate set once per test process and fails the test when it cannot.
func testTLSFixture(tb testing.TB) *tlsFixtureForTest {
	tb.Helper()
	tlsFixtureOnce.Do(func() {
		tlsFixtureValue, errTLSFixture = buildTLSFixtureForTest()
	})
	if errTLSFixture != nil {
		tb.Fatalf("tls fixture: %v", errTLSFixture)
	}
	return tlsFixtureValue
}

// buildTLSFixtureForTest generates the CA, the two server certificates, and the client certificate.
func buildTLSFixtureForTest() (*tlsFixtureForTest, error) {
	// CA: self-signed, valid for a day around now.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "simpleredis test CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}
	fixture := &tlsFixtureForTest{caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})}
	fixture.caPool = x509.NewCertPool()
	fixture.caPool.AddCert(caCert)

	// Leaves: server with IP SAN, server without, and the client.
	leaves := []struct {
		serial   int64
		name     string
		dnsNames []string
		ips      []net.IP
		usage    x509.ExtKeyUsage
		certPEM  *[]byte
		keyPEM   *[]byte
	}{
		{2, "simpleredis test server", []string{"localhost", "redis-tls", "dragonfly-tls"}, []net.IP{net.IPv4(127, 0, 0, 1)}, x509.ExtKeyUsageServerAuth, &fixture.serverPEM, &fixture.serverKeyPEM},
		{3, "simpleredis test server no ip", []string{"localhost"}, nil, x509.ExtKeyUsageServerAuth, &fixture.serverNoIPPEM, &fixture.serverNoIPKeyPEM},
		{4, "simpleredis test client", nil, nil, x509.ExtKeyUsageClientAuth, &fixture.clientPEM, &fixture.clientKeyPEM},
	}
	for _, leaf := range leaves {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(leaf.serial),
			Subject:      pkix.Name{CommonName: leaf.name},
			NotBefore:    notBefore,
			NotAfter:     notAfter,
			DNSNames:     leaf.dnsNames,
			IPAddresses:  leaf.ips,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{leaf.usage},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		if err != nil {
			return nil, err
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
		*leaf.certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		*leaf.keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	}
	return fixture, nil
}

// clientCertificate is the client key pair as tls.Config.Certificates expects it.
func (fixture *tlsFixtureForTest) clientCertificate(tb testing.TB) tls.Certificate {
	tb.Helper()
	pair, err := tls.X509KeyPair(fixture.clientPEM, fixture.clientKeyPEM)
	if err != nil {
		tb.Fatal(err)
	}
	return pair
}

// clientTLSConfig trusts the fixture CA, optionally presenting the client certificate.
func (fixture *tlsFixtureForTest) clientTLSConfig(tb testing.TB, withClientCert bool) *tls.Config {
	tb.Helper()
	config := &tls.Config{RootCAs: fixture.caPool, MinVersion: tls.VersionTLS12}
	if withClientCert {
		config.Certificates = []tls.Certificate{fixture.clientCertificate(tb)}
	}
	return config
}

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

// startSilentPeerForTest accepts TCP, never writes, and keeps each socket until the client closes.
// accepts is how many TCP connections it took.
func startSilentPeerForTest(tb testing.TB) (listenAddr string, accepts func() int) {
	tb.Helper()
	listener := listenLocalTCPForTest(tb)
	var mu sync.Mutex
	acceptCount := 0
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			acceptCount++
			mu.Unlock()
			go func(conn net.Conn) {
				defer conn.Close()
				drainUntilPeerCloseForTest(conn)
			}(conn)
		}
	}()
	return listener.Addr().String(), func() int {
		mu.Lock()
		defer mu.Unlock()
		return acceptCount
	}
}

// startWritesFirstPlainPeerForTest is a plain TCP server that answers the first bytes it reads (a ClientHello
// from a TLS client) with a RESP error line, then keeps the socket until the client closes.
func startWritesFirstPlainPeerForTest(tb testing.TB) (listenAddr string) {
	tb.Helper()
	listener := listenLocalTCPForTest(tb)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				firstBytes := make([]byte, 512)
				if _, err := conn.Read(firstBytes); err != nil {
					return
				}
				_, _ = io.WriteString(conn, "-ERR unknown command\r\n")
				drainUntilPeerCloseForTest(conn)
			}(conn)
		}
	}()
	return listener.Addr().String()
}

// TestWriteTLSFixtureFiles writes ca.crt, server.crt, server.key, client.crt, and client.key into the directory
// named by SIMPLEREDIS_TLS_FIXTURE_OUT, world-readable so a container user can load them. CI runs it before
// starting the live TLS engines. It skips when that variable is unset.
func TestWriteTLSFixtureFiles(t *testing.T) {
	outDir := os.Getenv(tlsFixtureOutEnv)
	if outDir == "" {
		t.Skip(tlsFixtureOutEnv + " unset")
	}
	fixture := testTLSFixture(t)
	if err := os.MkdirAll(outDir, 0o755); err != nil { //nolint:gosec // G301: the engine container reads these files
		t.Fatal(err)
	}
	files := map[string][]byte{
		"ca.crt":     fixture.caPEM,
		"server.crt": fixture.serverPEM,
		"server.key": fixture.serverKeyPEM,
		"client.crt": fixture.clientPEM,
		"client.key": fixture.clientKeyPEM,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(outDir, name), body, 0o644); err != nil { //nolint:gosec // G306: test keys for a container user
			t.Fatal(err)
		}
	}
	// The written pairs must load the way the engines and the live tests load them.
	if _, err := tls.LoadX509KeyPair(filepath.Join(outDir, "server.crt"), filepath.Join(outDir, "server.key")); err != nil {
		t.Fatalf("server pair: %v", err)
	}
	if _, err := tls.LoadX509KeyPair(filepath.Join(outDir, "client.crt"), filepath.Join(outDir, "client.key")); err != nil {
		t.Fatalf("client pair: %v", err)
	}
}
