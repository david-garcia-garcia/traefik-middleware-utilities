package iplookup

import (
	"math/rand"
	"net"
	"testing"
)

const hostMapStoreBenchN = 100000

func BenchmarkLoadIPv4Hosts(b *testing.B) {
	cidrs := randomHostCIDRsHostMapStore(false, hostMapStoreBenchN)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := New()
		for _, cidr := range cidrs {
			if err := h.AddCIDR(cidr, ""); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkLoadIPv6Hosts(b *testing.B) {
	cidrs := randomHostCIDRsHostMapStore(true, hostMapStoreBenchN)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := New()
		for _, cidr := range cidrs {
			if err := h.AddCIDR(cidr, ""); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkContainsIPv4Hit(b *testing.B) {
	h, hit := loadHostsHostMapStore(b, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, _, _, err := h.Contains(hit)
		if err != nil || !found {
			b.Fatalf("hit: found=%v err=%v", found, err)
		}
	}
}

func BenchmarkContainsIPv4Miss(b *testing.B) {
	h, _ := loadHostsHostMapStore(b, false)
	miss := net.ParseIP("240.0.0.1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, _, _, err := h.Contains(miss)
		if err != nil || found {
			b.Fatalf("miss: found=%v err=%v", found, err)
		}
	}
}

func BenchmarkContainsIPv6Hit(b *testing.B) {
	h, hit := loadHostsHostMapStore(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, _, _, err := h.Contains(hit)
		if err != nil || !found {
			b.Fatalf("hit: found=%v err=%v", found, err)
		}
	}
}

func BenchmarkContainsIPv6Miss(b *testing.B) {
	h, _ := loadHostsHostMapStore(b, true)
	miss := net.ParseIP("100::1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, _, _, err := h.Contains(miss)
		if err != nil || found {
			b.Fatalf("miss: found=%v err=%v", found, err)
		}
	}
}

func randomHostCIDRsHostMapStore(v6 bool, n int) []string {
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // G404: deterministic fixture
	cidrs := make([]string, n)
	for i := range cidrs {
		if v6 {
			cidrs[i] = randomIPv6HostCIDRHostMapStore(rng)
			continue
		}
		cidrs[i] = randomIPv4HostCIDRHostMapStore(rng)
	}
	return cidrs
}

func loadHostsHostMapStore(b *testing.B, v6 bool) (*Helper, net.IP) {
	b.Helper()
	cidrs := randomHostCIDRsHostMapStore(v6, hostMapStoreBenchN)
	h := New()
	for _, cidr := range cidrs {
		if err := h.AddCIDR(cidr, ""); err != nil {
			b.Fatal(err)
		}
	}
	_, last, err := net.ParseCIDR(cidrs[len(cidrs)-1])
	if err != nil {
		b.Fatal(err)
	}
	return h, last.IP
}
