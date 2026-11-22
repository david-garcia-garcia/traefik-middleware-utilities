package simpleredis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Live TLS engines: CI starts them with the TestWriteTLSFixtureFiles certificates, client certificates required,
// and ACL user liveTLSUser with password liveTLSUserPass (.github/workflows/ci.yml, jobs e2e-redis and e2e-dragonfly).
const (
	liveTLSRedisEnv     = "SIMPLEREDIS_LIVE_REDIS_TLS"
	liveTLSDragonflyEnv = "SIMPLEREDIS_LIVE_DRAGONFLY_TLS"
	liveTLSDirEnv       = "SIMPLEREDIS_LIVE_TLS_DIR"
	liveTLSUser         = "app"
	liveTLSUserPass     = "apppass"
)

// TestLiveTLS_MutualWithACLUser proves Set then Get over mutual TLS as ACL user app on each live TLS engine.
func TestLiveTLS_MutualWithACLUser(t *testing.T) {
	runForEachLiveEngine(t, liveTLSRedisEnv, liveTLSDragonflyEnv, func(t *testing.T, addr string) {
		t.Helper()
		client := newTestRedis(t, Config{Host: addr, TLS: liveTLSClientConfigForTest(t, true),
			User: liveTLSUser, Pass: liveTLSUserPass, CommandTimeout: 3 * time.Second})
		t.Cleanup(client.Close)
		key := "simpleredis-live-tls-" + t.Name()
		if err := client.Set(context.Background(), key, []byte("ok"), 60); err != nil {
			t.Fatalf("Set: %v", err)
		}
		got, err := client.Get(context.Background(), key)
		if err != nil || string(got) != "ok" {
			t.Fatalf("Get = %q, %v, want ok", got, err)
		}
	})
}

// TestLiveTLS_WrongACLPassword proves a wrong password for ACL user app over TLS is redis:noauth and not pooled.
func TestLiveTLS_WrongACLPassword(t *testing.T) {
	runForEachLiveEngine(t, liveTLSRedisEnv, liveTLSDragonflyEnv, func(t *testing.T, addr string) {
		t.Helper()
		client := newTestRedis(t, Config{Host: addr, TLS: liveTLSClientConfigForTest(t, true),
			User: liveTLSUser, Pass: "wrong-password", CommandTimeout: 3 * time.Second})
		t.Cleanup(client.Close)
		_, err := client.Get(context.Background(), "k")
		if err == nil || err.Error() != RedisNoAuth {
			t.Fatalf("Get = %v, want %s", err, RedisNoAuth)
		}
		if pooledIdle(client) != 0 {
			t.Fatalf("idle = %d, want 0", pooledIdle(client))
		}
	})
}

// TestLiveTLS_MissingClientCertificate proves an engine that requires a client certificate refuses one without it as redis:tls.
func TestLiveTLS_MissingClientCertificate(t *testing.T) {
	runForEachLiveEngine(t, liveTLSRedisEnv, liveTLSDragonflyEnv, func(t *testing.T, addr string) {
		t.Helper()
		client := newTestRedis(t, Config{Host: addr, TLS: liveTLSClientConfigForTest(t, false), CommandTimeout: 3 * time.Second})
		t.Cleanup(client.Close)
		_, err := client.Get(context.Background(), "k")
		if err == nil || err.Error() != RedisTLS {
			t.Fatalf("Get = %v, want %s", err, RedisTLS)
		}
	})
}

// TestLiveTLS_UntrustedServerCertificate proves a client whose RootCAs lacks the test CA gets redis:tls.
func TestLiveTLS_UntrustedServerCertificate(t *testing.T) {
	runForEachLiveEngine(t, liveTLSRedisEnv, liveTLSDragonflyEnv, func(t *testing.T, addr string) {
		t.Helper()
		config := liveTLSClientConfigForTest(t, true)
		config.RootCAs = x509.NewCertPool()
		client := newTestRedis(t, Config{Host: addr, TLS: config, User: liveTLSUser, Pass: liveTLSUserPass, CommandTimeout: 3 * time.Second})
		t.Cleanup(client.Close)
		_, err := client.Get(context.Background(), "k")
		if err == nil || err.Error() != RedisTLS {
			t.Fatalf("Get = %v, want %s", err, RedisTLS)
		}
	})
}

// liveTLSFileForTest reads one PEM file from SIMPLEREDIS_LIVE_TLS_DIR, failing when the directory is unset.
func liveTLSFileForTest(t *testing.T, name string) []byte {
	t.Helper()
	dir := os.Getenv(liveTLSDirEnv)
	if dir == "" {
		t.Fatalf("%s unset while a live TLS address is set", liveTLSDirEnv)
	}
	body, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// liveTLSClientConfigForTest trusts the live CA from SIMPLEREDIS_LIVE_TLS_DIR, optionally presenting the client certificate.
func liveTLSClientConfigForTest(t *testing.T, withClientCert bool) *tls.Config {
	t.Helper()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(liveTLSFileForTest(t, "ca.crt")) {
		t.Fatal("ca.crt holds no certificate")
	}
	config := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	if withClientCert {
		pair, err := tls.X509KeyPair(liveTLSFileForTest(t, "client.crt"), liveTLSFileForTest(t, "client.key"))
		if err != nil {
			t.Fatal(err)
		}
		config.Certificates = []tls.Certificate{pair}
	}
	return config
}
