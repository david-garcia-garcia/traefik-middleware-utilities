package iplookup

import (
	"net"
	"sync"
	"testing"
)

func mustContains(t *testing.T, h *Helper, ip string) (found bool, prefixLen int, metadata string) {
	t.Helper()
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("parse IP %q", ip)
	}
	found, prefixLen, metadata, err := h.Contains(parsed)
	if err != nil {
		t.Fatalf("Contains(%s): %v", ip, err)
	}
	return found, prefixLen, metadata
}

func TestFamilyIsolation(t *testing.T) {
	h := New()
	if err := h.AddCIDR("1.2.3.4/32", "v4"); err != nil {
		t.Fatal(err)
	}
	found, _, meta := mustContains(t, h, "102:304::1")
	if found {
		t.Fatalf("IPv4 /32 matched colliding IPv6; metadata=%q", meta)
	}
	found, prefixLen, meta := mustContains(t, h, "1.2.3.4")
	if !found || prefixLen != 32 || meta != "v4" {
		t.Fatalf("IPv4 hit: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}

	h = New()
	if err := h.AddCIDR("808:808::/32", "v6"); err != nil {
		t.Fatal(err)
	}
	found, _, meta = mustContains(t, h, "8.8.8.8")
	if found {
		t.Fatalf("IPv6 /32 matched colliding IPv4; metadata=%q", meta)
	}
	found, prefixLen, meta = mustContains(t, h, "808:808::1")
	if !found || prefixLen != 32 || meta != "v6" {
		t.Fatalf("IPv6 hit: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestIPv4MappedUsesIPv4Tree(t *testing.T) {
	h := New()
	if err := h.AddCIDR("8.8.8.8/32", "dns"); err != nil {
		t.Fatal(err)
	}
	found, prefixLen, meta := mustContains(t, h, "::ffff:8.8.8.8")
	if !found || prefixLen != 32 || meta != "dns" {
		t.Fatalf("mapped hit: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestCatchAllIsFamilyLocal(t *testing.T) {
	h := New()
	if err := h.AddCIDR("0.0.0.0/0", "all-v4"); err != nil {
		t.Fatal(err)
	}
	found, _, _ := mustContains(t, h, "2001:db8::1")
	if found {
		t.Fatal("IPv4 /0 matched IPv6")
	}
	found, prefixLen, meta := mustContains(t, h, "8.8.8.8")
	if !found || prefixLen != 0 || meta != "all-v4" {
		t.Fatalf("IPv4 /0: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}

	h = New()
	if err := h.AddCIDR("::/0", "all-v6"); err != nil {
		t.Fatal(err)
	}
	found, _, _ = mustContains(t, h, "8.8.8.8")
	if found {
		t.Fatal("IPv6 /0 matched IPv4")
	}
}

func TestAddCIDRCanonicalOverride(t *testing.T) {
	h := New()
	if err := h.AddCIDR("10.0.0.0/8", "office"); err != nil {
		t.Fatal(err)
	}
	if h.Count() != 1 {
		t.Fatalf("count after first insert = %d", h.Count())
	}
	if err := h.AddCIDR("10.0.0.1/8", "vpn"); err != nil {
		t.Fatal(err)
	}
	if h.Count() != 1 {
		t.Fatalf("count after override = %d", h.Count())
	}
	found, prefixLen, meta := mustContains(t, h, "10.1.2.3")
	if !found || prefixLen != 8 || meta != "vpn" {
		t.Fatalf("override: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestInvalidCIDR(t *testing.T) {
	h := New()
	if err := h.AddCIDR("not-a-cidr", "x"); err == nil {
		t.Fatal("expected parse error")
	}
	if h.Count() != 0 {
		t.Fatalf("count after failed add = %d", h.Count())
	}
	removed, err := h.RemoveCIDR("not-a-cidr")
	if err == nil || removed {
		t.Fatalf("remove invalid: removed=%v err=%v", removed, err)
	}
}

func TestRemoveOverlappingPrefixes(t *testing.T) {
	h := New()
	if err := h.AddCIDR("10.0.0.0/8", "wide"); err != nil {
		t.Fatal(err)
	}
	if err := h.AddCIDR("10.1.0.0/16", "narrow"); err != nil {
		t.Fatal(err)
	}
	removed, err := h.RemoveCIDR("10.0.0.0/8")
	if err != nil || !removed {
		t.Fatalf("remove /8: removed=%v err=%v", removed, err)
	}
	if h.Count() != 1 {
		t.Fatalf("count after remove = %d", h.Count())
	}
	found, _, _ := mustContains(t, h, "10.2.3.4")
	if found {
		t.Fatal("address only in /8 still matched")
	}
	found, prefixLen, meta := mustContains(t, h, "10.1.2.3")
	if !found || prefixLen != 16 || meta != "narrow" {
		t.Fatalf("remaining /16: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestRemoveMissingPrefix(t *testing.T) {
	h := New()
	removed, err := h.RemoveCIDR("192.0.2.0/24")
	if err != nil || removed {
		t.Fatalf("missing remove: removed=%v err=%v", removed, err)
	}
}

func TestResetClearsBothFamilies(t *testing.T) {
	h := New()
	if err := h.AddCIDR("192.0.2.0/24", "v4"); err != nil {
		t.Fatal(err)
	}
	if err := h.AddCIDR("2001:db8::/32", "v6"); err != nil {
		t.Fatal(err)
	}
	h.Reset()
	if h.Count() != 0 {
		t.Fatalf("count after reset = %d", h.Count())
	}
	found, _, _ := mustContains(t, h, "192.0.2.1")
	if found {
		t.Fatal("IPv4 still matched after reset")
	}
	found, _, _ = mustContains(t, h, "2001:db8::1")
	if found {
		t.Fatal("IPv6 still matched after reset")
	}
	h.Reset()
}

func TestLongestPrefixLabel(t *testing.T) {
	h := New()
	if err := h.AddCIDR("10.0.0.0/8", "wide"); err != nil {
		t.Fatal(err)
	}
	if err := h.AddCIDR("10.1.0.0/16", "narrow"); err != nil {
		t.Fatal(err)
	}
	found, prefixLen, meta := mustContains(t, h, "10.1.2.3")
	if !found || prefixLen != 16 || meta != "narrow" {
		t.Fatalf("longest: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
	found, prefixLen, meta = mustContains(t, h, "10.2.0.1")
	if !found || prefixLen != 8 || meta != "wide" {
		t.Fatalf("shorter: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestEmptyLabelAndMiss(t *testing.T) {
	h := New()
	if err := h.AddCIDR("192.0.2.0/24", ""); err != nil {
		t.Fatal(err)
	}
	found, _, meta := mustContains(t, h, "192.0.2.1")
	if !found || meta != "" {
		t.Fatalf("empty label: found=%v meta=%q", found, meta)
	}
	found, prefixLen, meta := mustContains(t, h, "203.0.113.1")
	if found || prefixLen != 0 || meta != "" {
		t.Fatalf("miss: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
}

func TestContainsNilIP(t *testing.T) {
	h := New()
	found, prefixLen, metadata, err := h.Contains(nil)
	if err == nil {
		t.Fatal("expected nil IP error")
	}
	_ = found
	_ = prefixLen
	_ = metadata
}

func TestConcurrentResetAndContains(t *testing.T) {
	h := New()
	if err := h.AddCIDR("10.0.0.0/8", "office"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		h.Reset()
	}()
	go func() {
		defer wg.Done()
		found, prefixLen, metadata, err := h.Contains(net.ParseIP("10.1.2.3"))
		_ = found
		_ = prefixLen
		_ = metadata
		_ = err
	}()
	wg.Wait()
	found, _, _ := mustContains(t, h, "10.1.2.3")
	if found {
		t.Fatal("Contains after Reset still found a prefix")
	}
}
