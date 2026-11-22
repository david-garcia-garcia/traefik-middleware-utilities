package iplookup

import (
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"testing"
)

const (
	hostMapStoreMemoryN      = 100000
	hostMapStoreIPv4MaxBytes = 75
	hostMapStoreIPv6MaxBytes = 100
	hostMapStoreLookupN      = 100000
	hostMapStoreMixHostsV4   = 200
	hostMapStoreMixHostsV6   = 200
	hostMapStoreMixMapped    = 50
	hostMapStoreMixShortV4   = 40
	hostMapStoreMixShortV6   = 40
)

// storedPrefixHostMapStore is one CIDR in the linear-scan reference matcher.
type storedPrefixHostMapStore struct {
	cidr    string
	network *net.IPNet
	label   string
}

func TestHostMapStoreEquivalence(t *testing.T) {
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // G404: deterministic fixture
	h := New()
	stored := seedMixHostMapStore(t, h, rng)
	assertLookupsMatchHostMapStore(t, h, stored, rng, hostMapStoreLookupN)

	stored = replaceRandomHostMapStore(t, h, stored, rng, len(stored)/4)
	assertLookupsMatchHostMapStore(t, h, stored, rng, hostMapStoreLookupN)

	stored = removeRandomHostMapStore(t, h, stored, rng, len(stored)/2)
	assertLookupsMatchHostMapStore(t, h, stored, rng, hostMapStoreLookupN)

	h.Reset()
	if h.Count() != 0 {
		t.Fatalf("count after Reset = %d, want 0", h.Count())
	}
	stored = nil
	assertLookupsMatchHostMapStore(t, h, stored, rng, 1000)
}

func TestHostMapStoreMemoryPerPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("heap bound is the full iplookup suite")
	}
	t.Run("IPv4", func(t *testing.T) {
		assertMemoryBoundHostMapStore(t, false, hostMapStoreIPv4MaxBytes)
	})
	t.Run("IPv6", func(t *testing.T) {
		assertMemoryBoundHostMapStore(t, true, hostMapStoreIPv6MaxBytes)
	})
}

func TestHostMapStoreLabelRefcount(t *testing.T) {
	h := New()
	hosts := []struct {
		cidr  string
		label string
	}{
		{"192.0.2.1/32", "office"},
		{"192.0.2.2/32", "office"},
		{"2001:db8::1/128", "vpn"},
		{"2001:db8::2/128", "vpn"},
		{"10.1.2.3/32", "unique"},
	}
	for _, host := range hosts {
		mustAddCIDRHostMapStore(t, h, host.cidr, host.label)
	}
	if h.Count() != len(hosts) {
		t.Fatalf("count after add = %d, want %d", h.Count(), len(hosts))
	}

	// Replace every host with a new label so old interned strings can drop.
	for i, host := range hosts {
		replaced := fmt.Sprintf("replaced-%d", i)
		mustAddCIDRHostMapStore(t, h, host.cidr, replaced)
	}
	if h.Count() != len(hosts) {
		t.Fatalf("count after replace = %d, want %d", h.Count(), len(hosts))
	}

	for _, host := range hosts {
		removed, err := h.RemoveCIDR(host.cidr)
		if err != nil || !removed {
			t.Fatalf("RemoveCIDR(%s): removed=%v err=%v", host.cidr, removed, err)
		}
	}
	if h.Count() != 0 {
		t.Fatalf("count after remove = %d, want 0", h.Count())
	}
	assertLabelTableOnlyEmptyHostMapStore(t, h)
}

func TestHostMapStoreHostBeatsShorterPrefix(t *testing.T) {
	h := New()
	mustAddCIDRHostMapStore(t, h, "10.0.0.0/8", "wide")
	mustAddCIDRHostMapStore(t, h, "10.1.2.3/32", "host")
	found, prefixLen, meta := mustContains(t, h, "10.1.2.3")
	if !found || prefixLen != 32 || meta != "host" {
		t.Fatalf("LPM host: found=%v prefix=%d meta=%q", found, prefixLen, meta)
	}
	if h.Count() != 2 {
		t.Fatalf("count = %d, want 2 (/32 and covering /8)", h.Count())
	}
}

func mustAddCIDRHostMapStore(t *testing.T, h *Helper, cidr, metadata string) {
	t.Helper()
	if err := h.AddCIDR(cidr, metadata); err != nil {
		t.Fatalf("AddCIDR(%s): %v", cidr, err)
	}
}

func seedMixHostMapStore(t *testing.T, h *Helper, rng *rand.Rand) []storedPrefixHostMapStore {
	t.Helper()
	var stored []storedPrefixHostMapStore
	add := func(cidr, label string) {
		t.Helper()
		before := h.Count()
		mustAddCIDRHostMapStore(t, h, cidr, label)
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%s): %v", cidr, err)
		}
		if h.Count() == before {
			for i := range stored {
				if stored[i].network.String() != network.String() {
					continue
				}
				stored[i].cidr = cidr
				stored[i].label = label
				return
			}
		}
		stored = append(stored, storedPrefixHostMapStore{cidr: cidr, network: network, label: label})
	}

	add("10.0.0.0/8", "wide-v4")
	add("10.1.0.0/16", "mid-v4")
	add("10.1.2.3/32", "host-in-wide")
	add("2001:db8::/32", "wide-v6")
	add("2001:db8:1::/48", "mid-v6")
	add("2001:db8:1::1/128", "host-in-wide-v6")
	add("::ffff:192.0.2.1/128", "mapped-host")

	for i := 0; i < hostMapStoreMixHostsV4; i++ {
		add(randomIPv4HostCIDRHostMapStore(rng), fmt.Sprintf("v4-%d", i))
	}
	for i := 0; i < hostMapStoreMixHostsV6; i++ {
		add(randomIPv6HostCIDRHostMapStore(rng), fmt.Sprintf("v6-%d", i))
	}
	for i := 0; i < hostMapStoreMixMapped; i++ {
		add(randomMappedHostCIDRHostMapStore(rng), fmt.Sprintf("mapped-%d", i))
	}
	for i := 0; i < hostMapStoreMixShortV4; i++ {
		add(randomIPv4PrefixCIDRHostMapStore(rng), fmt.Sprintf("short4-%d", i))
	}
	for i := 0; i < hostMapStoreMixShortV6; i++ {
		add(randomIPv6PrefixCIDRHostMapStore(rng), fmt.Sprintf("short6-%d", i))
	}
	return stored
}

func replaceRandomHostMapStore(t *testing.T, h *Helper, stored []storedPrefixHostMapStore, rng *rand.Rand, n int) []storedPrefixHostMapStore {
	t.Helper()
	if n > len(stored) {
		n = len(stored)
	}
	for i := 0; i < n; i++ {
		idx := rng.Intn(len(stored))
		label := fmt.Sprintf("replaced-%d-%d", i, rng.Intn(1<<20))
		mustAddCIDRHostMapStore(t, h, stored[idx].cidr, label)
		stored[idx].label = label
	}
	return stored
}

func removeRandomHostMapStore(t *testing.T, h *Helper, stored []storedPrefixHostMapStore, rng *rand.Rand, n int) []storedPrefixHostMapStore {
	t.Helper()
	if n > len(stored) {
		n = len(stored)
	}
	for i := 0; i < n; i++ {
		idx := rng.Intn(len(stored))
		removed, err := h.RemoveCIDR(stored[idx].cidr)
		if err != nil || !removed {
			t.Fatalf("RemoveCIDR(%s): removed=%v err=%v", stored[idx].cidr, removed, err)
		}
		stored[idx] = stored[len(stored)-1]
		stored = stored[:len(stored)-1]
	}
	return stored
}

func assertLookupsMatchHostMapStore(t *testing.T, h *Helper, stored []storedPrefixHostMapStore, rng *rand.Rand, n int) {
	t.Helper()
	if h.Count() != len(stored) {
		t.Fatalf("Count = %d, reference size = %d", h.Count(), len(stored))
	}
	for i := 0; i < n; i++ {
		ip := randomLookupIPHostMapStore(rng)
		wantFound, wantLen, wantLabel := containsRefHostMapStore(stored, ip)
		found, prefixLen, metadata, err := h.Contains(ip)
		if err != nil {
			t.Fatalf("Contains(%s): %v", ip, err)
		}
		if found != wantFound || prefixLen != wantLen || metadata != wantLabel {
			t.Fatalf("Contains(%s): found=%v prefix=%d meta=%q, want found=%v prefix=%d meta=%q",
				ip, found, prefixLen, metadata, wantFound, wantLen, wantLabel)
		}
	}
}

func containsRefHostMapStore(stored []storedPrefixHostMapStore, ip net.IP) (found bool, prefixLen int, label string) {
	bestLen := -1
	for _, prefix := range stored {
		covers, length := coversHostMapStore(prefix.network, ip)
		if !covers {
			continue
		}
		if !found || length > bestLen {
			found = true
			bestLen = length
			prefixLen = length
			label = prefix.label
		}
	}
	return found, prefixLen, label
}

func coversHostMapStore(cidr *net.IPNet, ip net.IP) (covers bool, prefixLen int) {
	if (cidr.IP.To4() != nil) != (ip.To4() != nil) {
		return false, 0
	}
	walkIP, maxLen := familyWalk(ip)
	cidrWalk, storedLen := prefixWalk(cidr)
	if storedLen > maxLen {
		return false, 0
	}
	for i := 0; i < storedLen; i++ {
		if bitAt(cidrWalk, i) != bitAt(walkIP, i) {
			return false, 0
		}
	}
	return true, storedLen
}

func randomLookupIPHostMapStore(rng *rand.Rand) net.IP {
	if rng.Intn(2) == 0 {
		return net.IPv4(byte(rng.Intn(223)+1), byte(rng.Intn(256)), byte(rng.Intn(256)), byte(rng.Intn(256)))
	}
	addr := make(net.IP, 16)
	rng.Read(addr)
	addr[0] = 0x20
	return addr
}

func randomIPv4HostCIDRHostMapStore(rng *rand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d/32", rng.Intn(223)+1, rng.Intn(256), rng.Intn(256), rng.Intn(256))
}

func randomIPv6HostCIDRHostMapStore(rng *rand.Rand) string {
	addr := make(net.IP, 16)
	rng.Read(addr)
	addr[0] = 0x20
	return addr.String() + "/128"
}

func randomMappedHostCIDRHostMapStore(rng *rand.Rand) string {
	return fmt.Sprintf("::ffff:%d.%d.%d.%d/128", rng.Intn(223)+1, rng.Intn(256), rng.Intn(256), rng.Intn(256))
}

func randomIPv4PrefixCIDRHostMapStore(rng *rand.Rand) string {
	ones := rng.Intn(24) + 1
	return fmt.Sprintf("%d.%d.%d.%d/%d", rng.Intn(223)+1, rng.Intn(256), rng.Intn(256), rng.Intn(256), ones)
}

func randomIPv6PrefixCIDRHostMapStore(rng *rand.Rand) string {
	ones := rng.Intn(96) + 1
	addr := make(net.IP, 16)
	rng.Read(addr)
	addr[0] = 0x20
	return fmt.Sprintf("%s/%d", addr.String(), ones)
}

func assertMemoryBoundHostMapStore(t *testing.T, v6 bool, maxBytesPerPrefix int) {
	t.Helper()
	rng := rand.New(rand.NewSource(1)) //nolint:gosec // G404: deterministic fixture
	cidrs := make([]string, hostMapStoreMemoryN)
	for i := range cidrs {
		if v6 {
			cidrs[i] = randomIPv6HostCIDRHostMapStore(rng)
			continue
		}
		cidrs[i] = randomIPv4HostCIDRHostMapStore(rng)
	}
	h := New()
	base := heapInuseHostMapStore()
	for _, cidr := range cidrs {
		if err := h.AddCIDR(cidr, ""); err != nil {
			t.Fatalf("AddCIDR: %v", err)
		}
	}
	after := heapInuseHostMapStore()
	runtime.KeepAlive(h)
	runtime.KeepAlive(cidrs)
	n := h.Count()
	if n == 0 {
		t.Fatal("no prefixes stored")
	}
	if after <= base {
		t.Fatalf("heap inuse did not grow (before=%d after=%d n=%d)", base, after, n)
	}
	used := after - base
	perPrefix := float64(used) / float64(n)
	t.Logf("v6=%v n=%d heap=%d (%.1f B/prefix) dest was 748/4952", v6, n, used, perPrefix)
	if perPrefix > float64(maxBytesPerPrefix) {
		t.Fatalf("%.1f B/prefix exceeds %d (old store is ~748 IPv4 / ~4952 IPv6)", perPrefix, maxBytesPerPrefix)
	}
}

func heapInuseHostMapStore() uint64 {
	prev := ^uint64(0)
	var stats runtime.MemStats
	for i := 0; i < 8; i++ {
		runtime.GC()
		runtime.ReadMemStats(&stats)
		if i >= 2 && stats.HeapInuse >= prev {
			return stats.HeapInuse
		}
		prev = stats.HeapInuse
	}
	return stats.HeapInuse
}

func assertLabelTableOnlyEmptyHostMapStore(t *testing.T, h *Helper) {
	t.Helper()
	if len(h.ipv4.hosts) != 0 || len(h.ipv6.hosts) != 0 {
		t.Fatalf("host maps still hold %d IPv4 and %d IPv6 entries", len(h.ipv4.hosts), len(h.ipv6.hosts))
	}
	if len(h.labels.id) != 1 {
		t.Fatalf("labelID size = %d, want only empty string", len(h.labels.id))
	}
	id, ok := h.labels.id[""]
	if !ok || id != 0 {
		t.Fatalf("labelID %v, want {\"\": 0}", h.labels.id)
	}
	for i, refs := range h.labels.refs {
		if i == 0 {
			continue
		}
		if refs != 0 {
			t.Fatalf("labelRefs[%d] = %d, want 0 after every host is gone", i, refs)
		}
	}
}
