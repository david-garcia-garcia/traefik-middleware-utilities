package backendbackoff

import (
	"time"
)

// Allow returns whether a backend attempt for key may proceed, plus how long to wait if not.
func (g *Gate) Allow(key string) (bool, time.Duration, error) {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false, 0, errClosed
	}
	entry := g.loadEntry(key, now)
	// Idle longer than ttl is a new key. Refresh so OPEN denies do not drop the map slot.
	entry.expireAt = now.Add(g.cfg.TTL)
	g.moveToBackLocked(entry)
	switch entry.state {
	case stateClosed:
		g.resetNIfClosedLongEnough(entry, now)
		return true, 0, nil
	case stateOpen:
		if now.Before(entry.openUntil) {
			return false, entry.openUntil.Sub(now), nil
		}
		entry.state = stateHalfOpen
		entry.probeOutstanding = true
		entry.probeUntil = now.Add(g.cfg.BaseCooldown)
		return true, 0, nil
	case stateHalfOpen:
		if entry.probeOutstanding && now.Before(entry.probeUntil) {
			return false, entry.probeUntil.Sub(now), nil
		}
		entry.probeOutstanding = true
		entry.probeUntil = now.Add(g.cfg.BaseCooldown)
		return true, 0, nil
	default:
		return false, 0, errUnknown
	}
}

// Report records the outcome of a real backend attempt the gate admitted.
func (g *Gate) Report(key string, success bool) error {
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return errClosed
	}
	entry := g.keys[key]
	if entry == nil {
		return nil
	}
	// Expire is Allow's job. An admitted attempt that Reports after TTL still lands.
	entry.expireAt = now.Add(g.cfg.TTL)
	g.moveToBackLocked(entry)
	switch entry.state {
	case stateOpen:
		return nil
	case stateHalfOpen:
		if !entry.probeOutstanding {
			return nil
		}
		entry.probeOutstanding = false
		if success {
			entry.credit = g.budget
			entry.state = stateClosed
			entry.closedSince = now
			return nil
		}
		entry.n++
		entry.state = stateOpen
		entry.openUntil = now.Add(g.cooldownDuration(entry.n))
		return nil
	case stateClosed:
		if success {
			entry.credit += g.successCredit()
			if entry.credit > g.budget {
				entry.credit = g.budget
			}
			return nil
		}
		g.resetNIfClosedLongEnough(entry, now)
		entry.credit--
		if entry.credit <= 0 {
			entry.state = stateOpen
			entry.openUntil = now.Add(g.cooldownDuration(entry.n))
			entry.closedSince = time.Time{}
		}
		return nil
	default:
		return errUnknown
	}
}

// loadEntry returns the key's slot, dropping it first when idle TTL has elapsed.
func (g *Gate) loadEntry(key string, now time.Time) *memEntry {
	entry := g.keys[key]
	if entry != nil && !now.Before(entry.expireAt) {
		g.removeLocked(entry)
		entry = nil
	}
	if entry == nil {
		g.makeRoomLocked(now)
		entry = &memEntry{
			credit:      g.budget,
			state:       stateClosed,
			closedSince: now,
			key:         key,
		}
		g.storeLocked(entry)
	}
	return entry
}

// resetNIfClosedLongEnough sets n to 0 after one MaxCooldown of continuous CLOSED.
func (g *Gate) resetNIfClosedLongEnough(entry *memEntry, now time.Time) {
	if entry.closedSince.IsZero() {
		return
	}
	if now.Before(entry.closedSince.Add(g.cfg.MaxCooldown)) {
		return
	}
	entry.n = 0
}

// makeRoomLocked removes expired keys from the head of the expiry list, then the head itself when the store is still at
// maxMemorySources, so a new key can be stored. Caller holds g.mu.
//
// If now moves backwards, a later-refreshed entry may sit behind one that expires sooner; lookup still treats a found
// expired key as unseen, and only eviction order is approximate. Production time.Now does not move backwards.
func (g *Gate) makeRoomLocked(now time.Time) {
	for g.head != nil && !now.Before(g.head.expireAt) {
		g.removeLocked(g.head)
	}
	if len(g.keys) >= maxMemorySources && g.head != nil {
		g.removeLocked(g.head)
	}
}

// storeLocked adds a new key to the map and at the tail of the expiry list. Caller holds g.mu.
func (g *Gate) storeLocked(entry *memEntry) {
	g.keys[entry.key] = entry
	g.pushBackLocked(entry)
}

// removeLocked deletes a stored key from the map and the expiry list. Caller holds g.mu.
func (g *Gate) removeLocked(entry *memEntry) {
	g.unlinkLocked(entry)
	delete(g.keys, entry.key)
}

// unlinkLocked takes a linked entry out of the expiry list. Caller holds g.mu.
func (g *Gate) unlinkLocked(entry *memEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		g.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		g.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

// moveToBackLocked moves a linked entry to the tail of the expiry list. The tail itself stays put, so a hot key costs no
// relink. Caller holds g.mu.
//
// It relinks in place instead of calling unlinkLocked then pushBackLocked: this runs on every Allow and Report of a stored
// key that is not the tail, and under Yaegi each method call costs microseconds.
func (g *Gate) moveToBackLocked(entry *memEntry) {
	if entry == g.tail {
		return
	}
	// Unlink. entry is not the tail, so entry.next is set.
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		g.head = entry.next
	}
	entry.next.prev = entry.prev
	// Append after the current tail.
	entry.prev = g.tail
	entry.next = nil
	g.tail.next = entry
	g.tail = entry
}

// pushBackLocked appends an unlinked entry at the tail of the expiry list. Caller holds g.mu.
func (g *Gate) pushBackLocked(entry *memEntry) {
	entry.prev = g.tail
	entry.next = nil
	if g.tail != nil {
		g.tail.next = entry
	} else {
		g.head = entry
	}
	g.tail = entry
}
