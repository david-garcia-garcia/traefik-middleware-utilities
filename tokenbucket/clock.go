// Package tokenbucket is a Yaegi-safe Traefik token-bucket clock (in-process and Redis EVAL).
package tokenbucket

import (
	"errors"
	"math"
	"time"
)

var (
	errRate       = errors.New("tokenbucket: rate must be greater than 0")
	errBurst      = errors.New("tokenbucket: burst must be at least 1")
	errDelay      = errors.New("tokenbucket: maxDelay must not be negative")
	errTTL        = errors.New("tokenbucket: ttl must be a whole number of seconds (at least 1s)")
	errRedis      = errors.New("tokenbucket: redis is required")
	errEvalLen    = errors.New("tokenbucket: eval reply must have 3 fields")
	errEvalWait   = errors.New("tokenbucket: eval wait is not a number")
	errEvalTokens = errors.New("tokenbucket: eval tokens is not a number")
)

// clockConfig is rate, burst, maxDelay, and ttl shared by both stores, plus the Status period derived from rate and burst.
type clockConfig struct {
	rate     float64
	burst    int64
	maxDelay time.Duration
	ttl      time.Duration
	period   time.Duration
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

// maxWholeSecondTTL is the largest time.Duration that is a whole number of seconds.
// A larger value overflows the nanosecond int64.
const maxWholeSecondTTL = time.Duration(math.MaxInt64/int64(time.Second)) * time.Second

// ttlSeconds is the Redis EXPIRE argument.
func (c clockConfig) ttlSeconds() int64 {
	return int64(c.ttl / time.Second)
}

// effectiveTTL raises caller to the whole seconds a bucket needs to refill from empty, including maxDelay.
// An expired bucket is then already full, so seeding it full grants nothing (traefik/traefik#13957).
// caller is already a whole number of seconds of at least 1s.
func effectiveTTL(rate float64, burst int64, maxDelay, caller time.Duration) time.Duration {
	floor := refillFloor(rate, burst, maxDelay)
	if caller > floor {
		return caller
	}
	return floor
}

// refillFloor is ceil(burst/rate + maxDelay seconds), clamped to maxWholeSecondTTL.
func refillFloor(rate float64, burst int64, maxDelay time.Duration) time.Duration {
	seconds := math.Ceil(float64(burst)/rate + maxDelay.Seconds())
	maxSeconds := float64(int64(maxWholeSecondTTL / time.Second))
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds > maxSeconds {
		return maxWholeSecondTTL
	}
	return time.Duration(int64(seconds)) * time.Second
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
