// Package leakybucket is a Yaegi-safe I.371 water-meter (in-process and Redis EVAL).
package leakybucket

import (
	"errors"
	"time"
)

var (
	errLeak      = errors.New("leakybucket: leak must be greater than 0")
	errCapacity  = errors.New("leakybucket: capacity must be greater than 0")
	errTTL       = errors.New("leakybucket: ttl must be at least 1s")
	errRedis     = errors.New("leakybucket: redis is required")
	errSyncRate  = errors.New("leakybucket: sync_rate must not be negative")
	errPour      = errors.New("leakybucket: n must be at least 1")
	errEvalLen   = errors.New("leakybucket: eval reply must have 3 fields")
	errEvalFlag  = errors.New("leakybucket: eval allowed is not 0 or 1")
	errEvalWater = errors.New("leakybucket: eval water is not a number")
	errEvalUntil = errors.New("leakybucket: eval until_not_full is not a number")
)

// clockConfig is leak, capacity, and ttl shared by both stores.
type clockConfig struct {
	leak     float64
	capacity float64
	ttl      time.Duration
}

// validateClock rejects leak, capacity, and ttl that would never drain or expire immediately.
func validateClock(leak, capacity float64, ttl time.Duration) error {
	if leak <= 0 {
		return errLeak
	}
	if capacity <= 0 {
		return errCapacity
	}
	if ttl < time.Second {
		return errTTL
	}
	return nil
}

// ttlSeconds is the Redis EXPIRE argument.
func (c clockConfig) ttlSeconds() int64 {
	return int64(c.ttl / time.Second)
}

// leakWater drains water from last to now. last and now are Unix microseconds. leak is water per second.
func leakWater(water float64, last int64, leakPerSecond float64, nowMicro int64) (float64, int64) {
	if nowMicro < last {
		last = nowMicro
	}
	elapsedSec := float64(nowMicro-last) / 1e6
	water = water - leakPerSecond*elapsedSec
	if water < 0 {
		water = 0
	}
	return water, nowMicro
}

// pourIfFits adds poured when it would not exceed capacity. On overflow it leaves water unchanged.
func pourIfFits(water, poured, capacity float64) (allowed bool, newWater float64) {
	if poured > 0 && water+poured > capacity {
		return false, water
	}
	return true, water + poured
}

// leakThenPour drains to now, then pours when the result would not exceed capacity.
func leakThenPour(water float64, last int64, leakPerSecond, capacity, poured float64, nowMicro int64) (allowed bool, newWater float64, newLast int64, until time.Duration) {
	leaked, newLast := leakWater(water, last, leakPerSecond, nowMicro)
	allowed, newWater = pourIfFits(leaked, poured, capacity)
	until = untilNotFull(newWater, capacity, leakPerSecond)
	return allowed, newWater, newLast, until
}

// untilNotFull is zero when there is already room for one more unit, else the time to leak down to capacity-1.
func untilNotFull(water, capacity, leakPerSecond float64) time.Duration {
	if water+1 <= capacity {
		return 0
	}
	untilMicro := (water - (capacity - 1)) / leakPerSecond * 1e6
	if untilMicro <= 0 {
		return 0
	}
	return time.Duration(untilMicro * float64(time.Microsecond))
}
