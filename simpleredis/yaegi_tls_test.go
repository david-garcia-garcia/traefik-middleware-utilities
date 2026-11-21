package simpleredis

import (
	"fmt"
	"testing"
)

// TestYaegi_TLSMutualWithACLUser proves interpreted New with Config.TLS (fixture CA and client certificate), User
// app, and Pass secret round-trips Set then Get against the TLS fake, which saw AUTH app secret over TLS.
// Traefik is not started.
func TestYaegi_TLSMutualWithACLUser(t *testing.T) {
	fixture := testTLSFixture(t)
	fake, state, addr := startTLSFakeRedis(t, map[string]string{}, tlsFakeOptionsForTest{requireClientCert: true})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)
	writeGopathClientprobeTLS(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.TLSRoundTrip(%q, %q, %q, %q, %q, %q, %q)`,
		addr, fixture.caPEM, fixture.clientPEM, fixture.clientKeyPEM, "app", "secret", "yaegi-tls-key"))
	if got != "ok" {
		t.Fatalf("yaegi TLS round-trip: %q, want ok", got)
	}
	if auth := fake.lastAuthArgs(); len(auth) != 2 || auth[0] != "app" || auth[1] != "secret" {
		t.Fatalf("AUTH args = %q, want [app secret]", auth)
	}
	if handshakes, clientCerts, _ := state.snapshot(); handshakes != 1 || clientCerts != 1 {
		t.Fatalf("handshakes=%d clientCerts=%d, want 1, 1", handshakes, clientCerts)
	}
}

// TestYaegi_TLSUnknownCAIsErrTLS proves interpreted errors.Is(err, ErrTLS) is true and IsUnreachable false for a
// server certificate the interpreted client does not trust.
func TestYaegi_TLSUnknownCAIsErrTLS(t *testing.T) {
	_, _, addr := startTLSFakeRedis(t, map[string]string{"hit": "t"}, tlsFakeOptionsForTest{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)
	writeGopathClientprobeTLS(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.TLSUnknownCA(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi TLS unknown CA: %q, want ok", got)
	}
}

// writeGopathClientprobeTLS adds the TLS probes to the interpreted clientprobe package.
func writeGopathClientprobeTLS(tb testing.TB, goPath string) {
	tb.Helper()
	writeGopathFile(tb, goPath, "clientprobe", "tls.go", clientprobeTLSSrc)
}

const clientprobeTLSSrc = `package clientprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// TLSRoundTrip builds a client that trusts caPEM, presents certPEM/keyPEM, and authenticates as user/pass,
// then Sets key and Gets it back.
func TLSRoundTrip(host, caPEM, certPEM, keyPEM, user, pass, key string) string {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(caPEM)) {
		return "ca: no certificate"
	}
	pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return "pair:" + err.Error()
	}
	client, err := simpleredis.New(simpleredis.Config{Host: host, User: user, Pass: pass,
		TLS: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}})
	if err != nil {
		return "new:" + err.Error()
	}
	defer client.Close()
	if err := client.Set(context.Background(), key, []byte("ok"), 60); err != nil {
		return "set:" + err.Error()
	}
	got, err := client.Get(context.Background(), key)
	if err != nil {
		return "get:" + err.Error()
	}
	return string(got)
}

// TLSUnknownCA reports errors.Is(err, ErrTLS) and not IsUnreachable for a server the client does not trust.
func TLSUnknownCA(host string) string {
	client, err := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: -1,
		TLS: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}})
	if err != nil {
		return "new:" + err.Error()
	}
	defer client.Close()
	_, err = client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if !errors.Is(err, simpleredis.ErrTLS) {
		return "errors.Is:" + err.Error()
	}
	if simpleredis.IsUnreachable(err) {
		return "IsUnreachable"
	}
	return "ok"
}
`
