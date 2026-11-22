# Test coverage

1. [hard] Edge case untested — `iplookup/store.go:87` — `unref` returns immediately on id 0 (`""` is never freed); no test removes or replaces a host stored with empty metadata, so deleting that guard leaves every test green
   Quote:
   ```go
   func (t *labelTable) unref(id uint32) {
   	if id == 0 {
   		return
   	}
   ```
   Note:
   ```
   grepped: TestHostMapStoreLabelRefcount (non-empty labels only); TestEmptyLabelAndMiss (trie /24, not intern); TestHostMapStoreMemoryPerPrefix (AddCIDR with "" but no Remove). (none) hits unref(0).
   ```
   Fix: Assert RemoveCIDR of a host stored with metadata "" drops the prefix, misses Contains, and leaves labels.id as {"": 0}
   Status: done
   Argument: TestHostMapStoreEmptyHostLabelRemove asserts remove, miss, and byLabel {"": 0}
2. [judgement] Happy path only — `iplookup/store.go:192` — `ipv6Store.contains` checks the host map first so a /128 beats a covering prefix; only the IPv4 sibling is asserted
   Quote:
   ```go
   func (s *ipv6Store) contains(ip net.IP, labels *labelTable) (found bool, prefixLen int, metadata string) {
   	walk, _ := familyWalk(ip)
   	if id, ok := s.hosts[ipv6HostKey(walk)]; ok {
   		return true, ipv6HostBits, labels.text[id]
   	}
   	return s.tree.contains(ip)
   }
   ```
   Note:
   ```
   grepped: TestHostMapStoreHostBeatsShorterPrefix (IPv4 /32 vs /8 only); TestYaegi_HostMapStoreHosts (IPv6 /128 with no covering prefix); TestHostMapStoreEquivalence seeds 2001:db8:1::1/128 under /32 but random lookups do not query that host.
   ```
   Fix: Assert Contains of a stored IPv6 /128 that also sits under a shorter prefix returns 128 and the host label
   Status: skipped
   Argument: judgement; IPv4 sibling plus equivalence mix already seed an IPv6 /128 under a covering prefix.
