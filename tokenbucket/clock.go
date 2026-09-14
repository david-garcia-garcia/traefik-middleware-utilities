// Package tokenbucket is a Yaegi-safe Traefik token-bucket clock (in-process and Redis EVAL).
package tokenbucket

import (
	"errors"
	"math"
	"time"
)

var (
	errRate     = errors.New("tokenbucket: rate must be greater than 0")
	errBurst    = errors.New("tokenbucket: burst must be at least 1")
	errDelay    = errors.New("tokenbucket: maxDelay must not be negative")
	errTTL      = errors.New("tokenbucket: ttl must be a whole number of seconds (at least 1s)")
	errRedis    = errors.New("tokenbucket: redis is required")
	errEvalLen  = errors.New("tokenbucket: eval reply must have 3 fields")
	errEvalWait = errors.New("tokenbucket: eval wait is not a number")
)

// clockConfig is rate, burst, maxDelay, and ttl shared by both stores.
type clockConfig struct {
	rate     float64
	burst    int64
	maxDelay time.Duration
	ttl      time.Duration
}

// validateClock rejects non-positive or non-finite rate, burst below 1, negative maxDelay, and ttl that would expire immediately or disagree with Redis EXPIRE seconds.
func validateClock(rate float64, burst int64, maxDelay, ttl time.Duration) error {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return errRate
	}
	if burst < 1 {
		return errBurst
	}
	if maxDelay < 0 {
		return errDelay
	}
	if ttl < time.Second || ttl%time.Second != 0 {
		return errTTL
	}
	return nil
}

// limitPerMicro is Traefik Redis ARGV limit: tokens per microsecond.
func (c clockConfig) limitPerMicro() float64 {
	return c.rate / 1e6
}

// ttlSeconds is the Redis EXPIRE argument.
func (c clockConfig) ttlSeconds() int64 {
	return int64(c.ttl / time.Second)
}

// consumeOne applies one Traefik Lua consume. last and now are Unix microseconds.
func consumeOne(tokens float64, last int64, limitPerMicro, burst float64, nowMicro, maxDelayMicro int64) (newTokens float64, newLast int64, waitMicro float64) {
	previousLast := last
	// Clamp last so a clock jump backward does not invent negative elapsed.
	if nowMicro < last {
		last = nowMicro
	}
	// Refill from last, cap at burst, then consume this Allow.
	elapsed := float64(nowMicro - last)
	tokens += limitPerMicro * elapsed
	if tokens > burst {
		tokens = burst
	}
	tokens--
	// Wait is how long until tokens reach 0. Refund when that wait exceeds maxDelay.
	if tokens < 0 {
		waitMicro = -tokens / limitPerMicro
		if waitMicro > float64(maxDelayMicro) {
			tokens++
			if tokens > burst {
				tokens = burst
			}
		}
	}
	// Persist the later of previous last and now so a backward clock does not rewind last.
	persistLast := nowMicro
	if previousLast > persistLast {
		persistLast = previousLast
	}
	return tokens, persistLast, waitMicro
}

// waitDuration converts Lua wait microseconds to a Go duration.
func waitDuration(waitMicro float64) time.Duration {
	if waitMicro <= 0 {
		return 0
	}
	return time.Duration(waitMicro * float64(time.Microsecond))
}

// allowedFromWait is true when wait is at most maxDelay (Lua refund leaves wait > maxDelay).
func allowedFromWait(wait, maxDelay time.Duration) bool {
	return wait <= maxDelay
}
