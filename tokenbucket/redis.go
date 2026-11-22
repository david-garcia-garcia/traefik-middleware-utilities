package tokenbucket

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// allowScriptDigest is Redis sha1hex of allowScript, computed once at package init.
var allowScriptDigest = simpleredis.ScriptSHA1Hex(allowScript)

// Redis is a Traefik token bucket stored as a Redis hash via Eval.
// Bucket state lives on the server. Any limiter that Allows the same key shares that budget, so the key prefix defines the scope.
type Redis struct {
	clock clockConfig
	now   func() time.Time
	redis *simpleredis.SimpleRedis
}

// NewRedis builds a Redis limiter on a SimpleRedis from simpleredis.New. rate is requests per second.
// Any *Redis on the same key shares the server budget; the caller prefixes the key to choose the scope.
func NewRedis(redis *simpleredis.SimpleRedis, rate float64, burst int64, maxDelay, ttl time.Duration) (*Redis, error) {
	if redis == nil {
		return nil, errRedis
	}
	if err := validateClock(rate, burst, maxDelay, ttl); err != nil {
		return nil, err
	}
	return &Redis{
		clock: clockConfig{
			rate:     rate,
			burst:    burst,
			maxDelay: maxDelay,
			ttl:      ttl,
			period:   refillPeriod(rate, burst),
		},
		now:   time.Now,
		redis: redis,
	}, nil
}

// SetNowForTest replaces the clock. Production callers must not use this.
func (r *Redis) SetNowForTest(now func() time.Time) {
	if now == nil {
		r.now = time.Now
		return
	}
	r.now = now
}

// Allow consumes one token for key via Eval and returns its Status: whether it is allowed, the wait as Delay or RetryAfter,
// and the bucket the script kept. The Status is the zero value when the error is non-nil.
func (r *Redis) Allow(ctx context.Context, key string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	nowMicro := r.now().UnixMicro()
	args := []string{
		strconv.FormatFloat(r.clock.limitPerMicro(), 'f', -1, 64),
		strconv.FormatInt(r.clock.burst, 10),
		strconv.FormatInt(r.clock.ttlSeconds(), 10),
		strconv.FormatInt(nowMicro, 10),
		strconv.FormatInt(r.clock.maxDelay.Microseconds(), 10),
	}
	// Script owns HGETALL/HSET/EXPIRE. Go maps its wait and tokens to a Status.
	values, err := r.redis.Eval(ctx, allowScript, allowScriptDigest, []string{key}, args)
	if err != nil {
		return Status{}, err
	}
	if len(values) != 3 {
		return Status{}, errEvalLen
	}
	// Wait is checked first, so a reply with both fields bad is still errEvalWait.
	waitMicro, ok := parseEvalNumber(values[1])
	if !ok {
		return Status{}, errEvalWait
	}
	tokens, ok := parseEvalNumber(values[2])
	if !ok {
		return Status{}, errEvalTokens
	}
	return newStatus(r.clock, tokens, waitMicro), nil
}

// parseEvalNumber reads one Eval reply field as a finite float. ParseFloat accepts nan and Inf with a nil error; those are not a
// number here.
func parseEvalNumber(field []byte) (number float64, ok bool) {
	number, err := strconv.ParseFloat(string(field), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}
