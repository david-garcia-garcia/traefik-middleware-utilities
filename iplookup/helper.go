// Package iplookup is a Yaegi-safe CIDR store. IPv4 and IPv6 prefixes live on
// separate family stores so a prefix cannot match the other family.
package iplookup

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Helper stores CIDR prefixes and returns the winning longest-prefix match.
// Callers pass net.IP they already selected; Helper does not read HTTP.
type Helper struct {
	mu     sync.RWMutex
	ipv4   ipv4Store
	ipv6   ipv6Store
	labels labelTable
	count  int
}

// New returns an empty Helper.
func New() *Helper {
	return &Helper{
		ipv4:   newIPv4Store(),
		ipv6:   newIPv6Store(),
		labels: newLabelTable(),
	}
}

// AddCIDR stores cidr on the family store for that network.
// A second store of the same canonical prefix replaces metadata and does not bump Count.
func (h *Helper) AddCIDR(cidr, metadata string) error {
	_, block, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("iplookup: parse CIDR %q: %w", cidr, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.insert(block, metadata) {
		h.count++
	}
	return nil
}

// AddEntry stores a CIDR or a bare address. A bare address becomes HostCIDR of
// that address, so an IPv4-mapped literal such as ::ffff:10.0.0.1 is the IPv4
// host 10.0.0.1/32, not an IPv6 /32. Surrounding whitespace is ignored.
func (h *Helper) AddEntry(entry, metadata string) error {
	cidr, err := entryCIDR(entry)
	if err != nil {
		return err
	}
	return h.AddCIDR(cidr, metadata)
}

// HostCIDR is the single-address prefix for ip: /32 for IPv4 and IPv4-mapped
// IPv6 (written as dotted IPv4), /128 for native IPv6.
func HostCIDR(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32"
	}
	return ip.String() + "/128"
}

// entryCIDR is entry as a CIDR string. Text with '/' is passed through for
// AddCIDR to parse; anything else must be one address.
func entryCIDR(entry string) (string, error) {
	trimmed := strings.TrimSpace(entry)
	if strings.IndexByte(trimmed, '/') >= 0 {
		return trimmed, nil
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return "", fmt.Errorf("iplookup: parse address or CIDR %q", entry)
	}
	return HostCIDR(ip), nil
}

// RemoveCIDR drops one stored prefix. Missing prefix returns false, nil error.
func (h *Helper) RemoveCIDR(cidr string) (removed bool, err error) {
	_, block, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, fmt.Errorf("iplookup: parse CIDR %q: %w", cidr, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.remove(block) {
		return false, nil
	}
	h.count--
	return true, nil
}

// Contains returns the longest stored prefix of the same family that covers ipAddr.
func (h *Helper) Contains(ipAddr net.IP) (found bool, prefixLen int, metadata string, err error) {
	if ipAddr == nil {
		return false, 0, "", errors.New("iplookup: IP address is nil")
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	found, prefixLen, metadata = h.lookup(ipAddr)
	return found, prefixLen, metadata, nil
}

// Reset drops every stored prefix on both family stores.
func (h *Helper) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ipv4 = newIPv4Store()
	h.ipv6 = newIPv6Store()
	h.labels = newLabelTable()
	h.count = 0
}

// Count is the number of stored prefixes, not the number of AddCIDR calls.
func (h *Helper) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}

// insert stores block on the IPv4 store when the network is IPv4 (including
// IPv4-mapped IPv6), otherwise on the IPv6 store.
func (h *Helper) insert(block *net.IPNet, metadata string) bool {
	if block.IP.To4() != nil {
		return h.ipv4.insert(block, metadata, &h.labels)
	}
	return h.ipv6.insert(block, metadata, &h.labels)
}

// remove drops block from the family store chosen by To4().
func (h *Helper) remove(block *net.IPNet) bool {
	if block.IP.To4() != nil {
		return h.ipv4.remove(block, &h.labels)
	}
	return h.ipv6.remove(block, &h.labels)
}

// lookup is Contains without the nil-IP check or lock.
func (h *Helper) lookup(ipAddr net.IP) (found bool, prefixLen int, metadata string) {
	if ipAddr.To4() != nil {
		return h.ipv4.contains(ipAddr, &h.labels)
	}
	return h.ipv6.contains(ipAddr, &h.labels)
}
