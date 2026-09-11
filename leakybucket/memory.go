package leakybucket

import (
	"sync"
	"time"
)

// memEntry is one in-memory key's water plus its lazy expire time.
type memEntry struct {
	water    float64
	last     int64
	expireAt time.Time
}

// Memory is an in-process leaky-bucket meter (exact water clock, no Redis).
type Memory struct {
	clock clockConfig
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*memEntry
}

// NewMemory builds an in-process meter. leak is water per second.
func NewMemory(leak, capacity float64, ttl time.Duration) (*Memory, error) {
	if err := validateClock(leak, capacity, ttl); err != nil {
		return nil, err
	}
	return &Memory{
		clock: clockConfig{
			leak:     leak,
			capacity: capacity,
			ttl:      ttl,
		},
		now:     time.Now,
		buckets: map[string]*memEntry{},
	}, nil
}

// SetNowForTest replaces the clock. Production callers must not use this.
func (m *Memory) SetNowForTest(now func() time.Time) {
	if now == nil {
		m.now = time.Now
		return
	}
	m.now = now
}

// Add pours n units for key and returns whether the pour is allowed, the water after leak (and pour when allowed), and until-not-full.
func (m *Memory) Add(key string, n int64) (bool, float64, time.Duration, error) {
	if n < 1 {
		return false, 0, 0, errPour
	}
	return m.pourLocked(key, float64(n))
}

// Take pours one unit for key.
func (m *Memory) Take(key string) (bool, float64, time.Duration, error) {
	return m.Add(key, 1)
}

// Level applies leak for key and returns the water. It does not pour.
func (m *Memory) Level(key string) (float64, error) {
	_, water, _, err := m.pourLocked(key, 0)
	return water, err
}

// pourLocked leaks then maybe pours under the map lock. poured 0 is Level.
func (m *Memory) pourLocked(key string, poured float64) (bool, float64, time.Duration, error) {
	now := m.now()
	nowMicro := now.UnixMicro()
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.buckets[key]
	if entry != nil && !now.Before(entry.expireAt) {
		delete(m.buckets, key)
		entry = nil
	}
	if entry == nil {
		m.dropExpired(now)
		entry = &memEntry{}
		m.buckets[key] = entry
	}
	allowed, water, last, until := leakThenPour(entry.water, entry.last, m.clock.leak, m.clock.capacity, poured, nowMicro)
	entry.water = water
	entry.last = last
	entry.expireAt = now.Add(m.clock.ttl)
	return allowed, water, until, nil
}

// dropExpired removes buckets whose ttl has elapsed.
func (m *Memory) dropExpired(now time.Time) {
	for source, entry := range m.buckets {
		if !now.Before(entry.expireAt) {
			delete(m.buckets, source)
		}
	}
}
