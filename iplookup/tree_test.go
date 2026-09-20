package iplookup

import (
	"net"
	"testing"
)

func TestFamilyWalkIPv4UsesFourByteForm(t *testing.T) {
	walk, maxPrefixLen := familyWalk(net.IP{192, 0, 2, 1})
	if len(walk) != 4 || maxPrefixLen != 32 {
		t.Fatalf("four-byte: len=%d max=%d", len(walk), maxPrefixLen)
	}

	mapped := net.ParseIP("192.0.2.1")
	walk, maxPrefixLen = familyWalk(mapped)
	if len(walk) != 4 || maxPrefixLen != 32 {
		t.Fatalf("mapped ParseIP: len=%d max=%d", len(walk), maxPrefixLen)
	}
}

func TestFamilyWalkIPv6Unchanged(t *testing.T) {
	ip := net.ParseIP("2001:db8::1")
	walk, maxPrefixLen := familyWalk(ip)
	if len(walk) != 16 || maxPrefixLen != 128 {
		t.Fatalf("v6: len=%d max=%d", len(walk), maxPrefixLen)
	}
}

func TestContainsIPv4FourByteNoAlloc(t *testing.T) {
	h := New()
	if err := h.AddCIDR("10.0.0.0/8", "office"); err != nil {
		t.Fatal(err)
	}
	query := net.IP{10, 1, 2, 3}
	allocs := testing.AllocsPerRun(1000, func() {
		found, _, _, err := h.Contains(query)
		if err != nil || !found {
			t.Fatalf("Contains: found=%v err=%v", found, err)
		}
	})
	if allocs != 0 {
		t.Fatalf("Contains 4-byte IPv4 allocated %v, want 0", allocs)
	}
}
