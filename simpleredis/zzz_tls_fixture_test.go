package simpleredis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
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
