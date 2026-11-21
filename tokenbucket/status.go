package tokenbucket

import (
	"math"
	"time"
)

// maxDurationNanos is the largest time.Duration as a float; a float duration at or above it saturates.
const maxDurationNanos = float64(math.MaxInt64)

// Status is the outcome of one Allow: whether the consume is allowed, plus the bucket values a caller needs to write
// rate-limit response headers. The library never writes HTTP and never sleeps.
type Status struct {
	// Allowed is true when the wait in microseconds is at most maxDelay truncated to whole microseconds.
	Allowed bool
	// Limit is burst.
	Limit int64
	// Remaining is the whole tokens the store keeps after this Allow (after the refund when denied), never above Limit.
	Remaining int64
	// Period is burst / rate, the time to refill an empty bucket. It is derived: the library has no configured period.
	Period time.Duration
	// Reset is the time until the bucket is full again.
	Reset time.Duration
	// RetryAfter is the wait when denied (the full wait, not wait minus maxDelay), and zero when allowed.
	RetryAfter time.Duration
	// Delay is the wait when allowed, which the caller sleeps to keep Traefik's delayed admit, and zero when denied.
	// Delay + RetryAfter is the wait the store computed.
	Delay time.Duration
}

// newStatus builds the Status of one Allow from the tokens the store keeps after it and the wait in microseconds. Memory and
// Redis both call it, so every field is computed the same way on both stores.
//
// It is one body on purpose: it runs on every Allow, and under Yaegi each interpreted call costs about a microsecond
// (BenchmarkYaegiMemoryAllow), so Period comes precomputed from the clock and the remaining, reset, and wait conversions are
// written inline with the rule of durationFromNanos.
func newStatus(clock clockConfig, tokens, waitMicro float64) Status {
	burst := float64(clock.burst)
	// Admit uses whole microseconds, same units as consumeOne refund and Lua ARGV.
	status := Status{
		Allowed: waitMicro <= float64(clock.maxDelay.Microseconds()),
		Limit:   clock.burst,
		Period:  clock.period,
	}
	// Remaining is floor(tokens) clamped to [0, burst]: the clamp holds when Redis's 14-digit tokens text or float64(burst)
	// rounds above burst; int64 truncation is floor for tokens >= 1; NaN fails both comparisons and stays 0.
	if tokens >= burst {
		status.Remaining = clock.burst
	} else if tokens >= 1 {
		status.Remaining = int64(tokens)
	}
	// Reset is the time to refill burst - tokens, saturating and never negative.
	resetNanos := (burst - tokens) * float64(time.Second) / clock.rate
	if resetNanos >= maxDurationNanos {
		status.Reset = math.MaxInt64
	} else if resetNanos > 0 {
		status.Reset = time.Duration(resetNanos)
	}
	// The same wait is a delay on admit and a retry-after on deny, saturating and never negative.
	waitNanos := waitMicro * float64(time.Microsecond)
	wait := time.Duration(0)
	if waitNanos >= maxDurationNanos {
		wait = math.MaxInt64
	} else if waitNanos > 0 {
		wait = time.Duration(waitNanos)
	}
	if status.Allowed {
		status.Delay = wait
	} else {
		status.RetryAfter = wait
	}
	return status
}

// refillPeriod is burst / rate as a duration, the time to refill an empty bucket, computed once per limiter. It multiplies
// before dividing so a whole result stays exact (5 tokens at 10/s is 500ms); dividing by limitPerMicro gives 499.999999ms.
func refillPeriod(rate float64, burst int64) time.Duration {
	return durationFromNanos(float64(burst) * float64(time.Second) / rate)
}

// durationFromNanos converts float nanoseconds to a duration: zero when not positive (or NaN), the maximum duration when
// larger than a duration can hold. newStatus inlines the same rule.
func durationFromNanos(nanos float64) time.Duration {
	if nanos >= maxDurationNanos {
		return math.MaxInt64
	}
	if nanos > 0 {
		return time.Duration(nanos)
	}
	return 0
}
