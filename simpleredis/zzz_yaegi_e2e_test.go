package simpleredis

import (
	"fmt"
	"testing"
)

// TestYaegiLive_Engines runs interpreted New plus every public verb against each live engine.
func TestYaegiLive_Engines(t *testing.T) {
	runForEachLiveEngine(t, "", func(t *testing.T, addr string) {
		t.Helper()
		client := waitLiveSimpleRedis(t, addr)
		t.Cleanup(client.Close)
		goPath := t.TempDir()
		writeGopathSimpleredis(t, goPath)
		writeGopathClientprobe(t, goPath)
		got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.LiveVerbs(%q, %q)`, addr, t.Name()))
		if got != "ok" {
			t.Fatalf("yaegi live verbs: %q, want ok", got)
		}
	})
}

// TestYaegiLive_TLSEngines runs interpreted New with Config.TLS (test CA and client certificate) and ACL
// user app, plus Set and Get, against each live TLS engine.
func TestYaegiLive_TLSEngines(t *testing.T) {
	runForEachLiveEngine(t, liveTLSSuffix, func(t *testing.T, addr string) {
		t.Helper()
		goPath := t.TempDir()
		writeGopathSimpleredis(t, goPath)
		writeGopathClientprobe(t, goPath)
		writeGopathClientprobeTLS(t, goPath)
		got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.TLSRoundTrip(%q, %q, %q, %q, %q, %q, %q)`,
			addr, liveTLSFileForTest(t, "ca.crt"), liveTLSFileForTest(t, "client.crt"), liveTLSFileForTest(t, "client.key"),
			liveTLSUser, liveTLSUserPass, "simpleredis-yaegi-live-tls-"+t.Name()))
		if got != "ok" {
			t.Fatalf("yaegi live TLS: %q, want ok", got)
		}
	})
}
