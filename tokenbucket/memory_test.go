package tokenbucket

// storeMemEntryForTest stores a hand-built entry under key the way Allow stores a new source, so tests and benchmarks can prefill a Memory without calling Allow.
func storeMemEntryForTest(m *Memory, key string, entry *memEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buckets[key] = entry
}
