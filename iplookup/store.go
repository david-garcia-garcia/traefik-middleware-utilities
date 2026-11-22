package iplookup

import "net"

const (
	ipv4HostBits = 32
	ipv6HostBits = 128
)

// ipv4Store is the IPv4 family store: host /32 entries in a pointer-free map
// and shorter prefixes on the existing radix trie.
type ipv4Store struct {
	hosts map[uint32]uint32
	tree  *ipRadixTree
}

// ipv6Store is the IPv6 family store: host /128 entries keyed by the 16-byte
// address and shorter prefixes on the existing radix trie.
type ipv6Store struct {
	hosts map[[16]byte]uint32
	tree  *ipRadixTree
}

// labelTable interns match labels so host maps hold ids instead of strings.
// Id 0 is "" and is never freed.
type labelTable struct {
	id   map[string]uint32
	text []string
	refs []uint32
	free []uint32
}

// newIPv4Store returns an empty IPv4 family store.
func newIPv4Store() ipv4Store {
	return ipv4Store{
		hosts: make(map[uint32]uint32),
		tree:  newIPRadixTree(),
	}
}

// newIPv6Store returns an empty IPv6 family store.
func newIPv6Store() ipv6Store {
	return ipv6Store{
		hosts: make(map[[16]byte]uint32),
		tree:  newIPRadixTree(),
	}
}

// newLabelTable returns a table whose only interned label is "".
func newLabelTable() labelTable {
	return labelTable{
		id:   map[string]uint32{"": 0},
		text: []string{""},
		refs: []uint32{1},
	}
}

// intern returns the id for label, incrementing its ref. Id 0 is never freed.
func (t *labelTable) intern(label string) uint32 {
	if label == "" {
		return 0
	}
	if id, ok := t.id[label]; ok {
		t.refs[id]++
		return id
	}

	// Reuse a freed slot so the table does not grow without bound on churn.
	if n := len(t.free); n > 0 {
		id := t.free[n-1]
		t.free = t.free[:n-1]
		t.id[label] = id
		t.text[id] = label
		t.refs[id] = 1
		return id
	}

	id := uint32(len(t.text)) //nolint:gosec // G115: distinct labels on one Helper, not a wire length
	t.id[label] = id
	t.text = append(t.text, label)
	t.refs = append(t.refs, 1)
	return id
}

// unref drops one live use of id. Id 0 is never freed.
func (t *labelTable) unref(id uint32) {
	if id == 0 {
		return
	}
	t.refs[id]--
	if t.refs[id] > 0 {
		return
	}
	delete(t.id, t.text[id])
	t.text[id] = ""
	t.free = append(t.free, id)
}

// ipv4HostKey is the big-endian uint32 of a 4-byte IPv4 walk.
func ipv4HostKey(walk net.IP) uint32 {
	return uint32(walk[0])<<24 | uint32(walk[1])<<16 | uint32(walk[2])<<8 | uint32(walk[3])
}

// ipv6HostKey copies a 16-byte IPv6 walk onto the stack. No heap allocation.
func ipv6HostKey(walk net.IP) (key [16]byte) {
	copy(key[:], walk)
	return key
}

// insert stores cidr. Host length goes to the map; anything shorter goes to the trie.
func (s *ipv4Store) insert(cidr *net.IPNet, metadata string, labels *labelTable) (added bool) {
	walk, prefixLen := prefixWalk(cidr)
	if prefixLen != ipv4HostBits {
		return s.tree.insert(cidr, metadata)
	}
	return s.putHost(ipv4HostKey(walk), labels.intern(metadata), labels)
}

// insert stores cidr. Host length goes to the map; anything shorter goes to the trie.
func (s *ipv6Store) insert(cidr *net.IPNet, metadata string, labels *labelTable) (added bool) {
	walk, prefixLen := prefixWalk(cidr)
	if prefixLen != ipv6HostBits {
		return s.tree.insert(cidr, metadata)
	}
	return s.putHost(ipv6HostKey(walk), labels.intern(metadata), labels)
}

// putHost writes id at key. A second store of the same host replaces the label.
func (s *ipv4Store) putHost(key uint32, id uint32, labels *labelTable) bool {
	if oldID, exists := s.hosts[key]; exists {
		s.hosts[key] = id
		labels.unref(oldID)
		return false
	}
	s.hosts[key] = id
	return true
}

// putHost writes id at key. A second store of the same host replaces the label.
func (s *ipv6Store) putHost(key [16]byte, id uint32, labels *labelTable) bool {
	if oldID, exists := s.hosts[key]; exists {
		s.hosts[key] = id
		labels.unref(oldID)
		return false
	}
	s.hosts[key] = id
	return true
}

// remove drops cidr. Missing prefix returns false.
func (s *ipv4Store) remove(cidr *net.IPNet, labels *labelTable) bool {
	walk, prefixLen := prefixWalk(cidr)
	if prefixLen != ipv4HostBits {
		return s.tree.remove(cidr)
	}
	key := ipv4HostKey(walk)
	id, ok := s.hosts[key]
	if !ok {
		return false
	}
	delete(s.hosts, key)
	labels.unref(id)
	return true
}

// remove drops cidr. Missing prefix returns false.
func (s *ipv6Store) remove(cidr *net.IPNet, labels *labelTable) bool {
	walk, prefixLen := prefixWalk(cidr)
	if prefixLen != ipv6HostBits {
		return s.tree.remove(cidr)
	}
	key := ipv6HostKey(walk)
	id, ok := s.hosts[key]
	if !ok {
		return false
	}
	delete(s.hosts, key)
	labels.unref(id)
	return true
}

// contains returns a host hit first (longest possible prefix), else the trie match.
func (s *ipv4Store) contains(ip net.IP, labels *labelTable) (found bool, prefixLen int, metadata string) {
	walk, _ := familyWalk(ip)
	if id, ok := s.hosts[ipv4HostKey(walk)]; ok {
		return true, ipv4HostBits, labels.text[id]
	}
	return s.tree.contains(ip)
}

// contains returns a host hit first (longest possible prefix), else the trie match.
func (s *ipv6Store) contains(ip net.IP, labels *labelTable) (found bool, prefixLen int, metadata string) {
	walk, _ := familyWalk(ip)
	if id, ok := s.hosts[ipv6HostKey(walk)]; ok {
		return true, ipv6HostBits, labels.text[id]
	}
	return s.tree.contains(ip)
}
