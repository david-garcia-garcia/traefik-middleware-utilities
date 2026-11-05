package iplookup

import (
	"net"
	"strings"
	"testing"
)

func TestHostCIDR(t *testing.T) {
	tests := []struct {
		ip   string
		want string
	}{
		{"10.0.0.1", "10.0.0.1/32"},
		{"::ffff:10.0.0.1", "10.0.0.1/32"},
		{"2001:db8::1", "2001:db8::1/128"},
		{"::1", "::1/128"},
	}
	for _, tt := range tests {
		if got := HostCIDR(net.ParseIP(tt.ip)); got != tt.want {
			t.Errorf("HostCIDR(%s) = %q, want %q", tt.ip, got, tt.want)
		}
	}
}

func TestAddEntryBareAddressIsHostPrefix(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		hit   []string
		miss  []string
	}{
		{"IPv4", "10.10.60.5", []string{"10.10.60.5", "::ffff:10.10.60.5"}, []string{"10.10.60.6", "::5"}},
		{"IPv4 with spaces", "  10.10.60.5 ", []string{"10.10.60.5"}, []string{"10.10.60.6"}},
		{"IPv6", "2001:db8::1", []string{"2001:db8::1"}, []string{"2001:db8::2"}},
		// The mapped literal is the IPv4 host, not ::/32 on the IPv6 tree.
		{"IPv4-mapped", "::ffff:10.10.60.5", []string{"10.10.60.5"}, []string{"10.10.60.6", "::5", "::1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			if err := h.AddEntry(tt.entry, "proxy"); err != nil {
				t.Fatalf("AddEntry(%q): %v", tt.entry, err)
			}
			for _, ip := range tt.hit {
				found, prefixLen, meta := mustContains(t, h, ip)
				if !found || meta != "proxy" {
					t.Errorf("Contains(%s) found=%v meta=%q, want hit", ip, found, meta)
				}
				wantLen := 32
				if net.ParseIP(ip).To4() == nil {
					wantLen = 128
				}
				if prefixLen != wantLen {
					t.Errorf("Contains(%s) prefixLen=%d, want %d", ip, prefixLen, wantLen)
				}
			}
			for _, ip := range tt.miss {
				if found, _, _ := mustContains(t, h, ip); found {
					t.Errorf("Contains(%s) = true, want miss", ip)
				}
			}
		})
	}
}

func TestAddEntryCIDRMatchesAddCIDR(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0/8", "::ffff:10.0.0.0/104", "2001:db8::/32"} {
		h := New()
		if err := h.AddEntry(" "+cidr+" ", "a"); err != nil {
			t.Fatalf("AddEntry(%q): %v", cidr, err)
		}
		if err := h.AddCIDR(cidr, "b"); err != nil {
			t.Fatalf("AddCIDR(%q): %v", cidr, err)
		}
		if h.Count() != 1 {
			t.Errorf("%s: AddEntry and AddCIDR stored %d prefixes, want the same one", cidr, h.Count())
		}
	}
}

func TestAddEntryRejectsInvalid(t *testing.T) {
	for _, entry := range []string{"", "   ", "not-an-ip", "10.0.0.0/33", "10.0.0.1:443"} {
		h := New()
		err := h.AddEntry(entry, "x")
		if err == nil {
			t.Errorf("AddEntry(%q): want error", entry)
			continue
		}
		if !strings.Contains(err.Error(), strings.TrimSpace(entry)) {
			t.Errorf("AddEntry(%q) error %q should name the entry", entry, err)
		}
		if h.Count() != 0 {
			t.Errorf("AddEntry(%q) stored %d prefixes after an error", entry, h.Count())
		}
	}
}
