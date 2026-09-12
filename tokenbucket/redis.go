package tokenbucket

import (
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// Redis is a Traefik token bucket stored as a Redis hash via Eval.
type Redis struct {
	clock clockConfig
	now   func() time.Time
	redis *simpleredis.SimpleRedis
}

// NewRedis builds a Redis limiter on a SimpleRedis from simpleredis.New. rate is requests per second.
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

// Allow consumes one token for key via Eval and returns whether it is allowed plus how long to wait.
func (r *Redis) Allow(key string) (bool, time.Duration, error) {
	nowMicro := r.now().UnixMicro()
	args := []string{
		strconv.FormatFloat(r.clock.limitPerMicro(), 'f', -1, 64),
		strconv.FormatInt(r.clock.burst, 10),
		strconv.FormatInt(r.clock.ttlSeconds(), 10),
		strconv.FormatInt(nowMicro, 10),
		strconv.FormatInt(r.clock.maxDelay.Microseconds(), 10),
	}
	// Script owns HGETALL/HSET/EXPIRE. Go only maps wait to allowed.
	values, err := r.redis.Eval(allowScript, []string{key}, args)
	if err != nil {
		return false, 0, err
	}
	if len(values) != 3 {
		return false, 0, errEvalLen
	}
	waitMicro, convErr := strconv.ParseFloat(string(values[1]), 64)
	if convErr != nil {
		return false, 0, errEvalWait
	}
	wait := waitDuration(waitMicro)
	return allowedFromWait(wait, r.clock.maxDelay), wait, nil
}
