package leakybucket

import (
	"strconv"
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// minSyncRate is Kong Advanced's floor for a positive sync interval.
const minSyncRate = 20 * time.Millisecond

// Redis is a leaky-bucket meter stored as a Redis hash via Eval.
type Redis struct {
	clock    clockConfig
	now      func() time.Time
	redis    *simpleredis.SimpleRedis
	syncRate time.Duration

	mu     sync.Mutex
	keys   map[string]*keyState
	closed bool
	ticker *time.Ticker
	stop   chan struct{}
	wg     sync.WaitGroup
}

// keyState is the buffered pours for one Redis hash key.
type keyState struct {
	redisWater float64
	lastSync   int64
	localPours float64
	expireAt   time.Time
}

// NewRedis builds a Redis meter on an already-Init SimpleRedis. leak is water per second. Negative syncRate fails. Positive values below 20ms floor to 20ms. Zero is exact (EVAL every Add).
func NewRedis(redis *simpleredis.SimpleRedis, leak, capacity float64, syncRate, ttl time.Duration) (*Redis, error) {
	if redis == nil {
		return nil, errRedis
	}
	if syncRate < 0 {
		return nil, errSyncRate
	}
	if err := validateClock(leak, capacity, ttl); err != nil {
		return nil, err
	}
	if syncRate > 0 && syncRate < minSyncRate {
		syncRate = minSyncRate
	}
	limiter := &Redis{
		clock: clockConfig{
			leak:     leak,
			capacity: capacity,
			ttl:      ttl,
		},
		now:      time.Now,
		redis:    redis,
		syncRate: syncRate,
		keys:     map[string]*keyState{},
	}
	if syncRate > 0 {
		limiter.startFlushLocked()
	}
	return limiter, nil
}

// SetNowForTest replaces the clock. Production callers must not use this.
func (r *Redis) SetNowForTest(now func() time.Time) {
	if now == nil {
		r.now = time.Now
		return
	}
	r.now = now
}

// Add pours n units for key and returns whether the pour is allowed, the water after leak (and pour when allowed), and until-not-full.
func (r *Redis) Add(key string, n int64) (bool, float64, time.Duration, error) {
	if n < 1 {
		return false, 0, 0, errPour
	}
	if r.syncRate == 0 {
		return r.evalPour(key, float64(n))
	}
	return r.addBuffered(key, float64(n))
}

// Take pours one unit for key.
func (r *Redis) Take(key string) (bool, float64, time.Duration, error) {
	return r.Add(key, 1)
}

// Level applies leak for key and returns the water. It does not pour.
func (r *Redis) Level(key string) (float64, error) {
	if r.syncRate == 0 {
		_, water, _, err := r.evalPour(key, 0)
		return water, err
	}
	return r.levelBuffered(key)
}

// evalPour runs the leak-then-add script. poured 0 is Level.
func (r *Redis) evalPour(key string, poured float64) (bool, float64, time.Duration, error) {
	nowMicro := r.now().UnixMicro()
	values, err := r.redis.Eval(pourScript, []string{key}, []string{
		strconv.FormatFloat(r.clock.leak, 'f', -1, 64),
		strconv.FormatFloat(r.clock.capacity, 'f', -1, 64),
		strconv.FormatInt(r.clock.ttlSeconds(), 10),
		strconv.FormatInt(nowMicro, 10),
		strconv.FormatFloat(poured, 'f', -1, 64),
	})
	if err != nil {
		return false, 0, 0, err
	}
	return parseEvalReply(values)
}

// addBuffered admits from leaked Redis water plus local pours and leaves Redis to the flush ticker.
func (r *Redis) addBuffered(key string, poured float64) (bool, float64, time.Duration, error) {
	now := r.now()
	nowMicro := now.UnixMicro()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.keyLocked(key, now)
	leaked, _ := leakWater(state.redisWater, state.lastSync, r.clock.leak, nowMicro)
	estimate := leaked + state.localPours
	allowed, newWater := pourIfFits(estimate, poured, r.clock.capacity)
	if allowed {
		state.localPours += poured
	}
	state.expireAt = now.Add(r.clock.ttl)
	return allowed, newWater, untilNotFull(newWater, r.clock.capacity, r.clock.leak), nil
}

// levelBuffered returns leaked Redis water plus local pours without pouring.
func (r *Redis) levelBuffered(key string) (float64, error) {
	now := r.now()
	nowMicro := now.UnixMicro()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.keyLocked(key, now)
	leaked, _ := leakWater(state.redisWater, state.lastSync, r.clock.leak, nowMicro)
	return leaked + state.localPours, nil
}

// keyLocked returns the buffer for a Redis key, creating an empty one on first sight.
func (r *Redis) keyLocked(key string, now time.Time) *keyState {
	state := r.keys[key]
	if state != nil {
		return state
	}
	state = &keyState{expireAt: now.Add(r.clock.ttl)}
	r.keys[key] = state
	return state
}

// Sleep flushes pending pours then stops the flush ticker. Exact mode is a no-op besides stopping leftover work.
func (r *Redis) Sleep() {
	_ = r.flushPending()
	r.stopFlushAndWait()
}

// Wake starts the flush ticker when sync_rate is positive and the limiter is not closed.
func (r *Redis) Wake() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.syncRate == 0 || r.stop != nil {
		return
	}
	r.startFlushLocked()
}

// Close stops the ticker after Sleep and refuses to start another. It does not close the injected SimpleRedis.
func (r *Redis) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()
	_ = r.flushPending()
	r.stopFlushAndWait()
}

// startFlushLocked starts the ticker goroutine. Caller holds r.mu (or is still unpublished). NewTicker is not time.Tick.
func (r *Redis) startFlushLocked() {
	r.stop = make(chan struct{})
	r.ticker = time.NewTicker(r.syncRate)
	r.wg.Add(1)
	go r.flushLoop(r.ticker, r.stop)
}

// takeFlushTickerLocked closes the stop channel and detaches the ticker. Caller holds r.mu. Does not wait.
func (r *Redis) takeFlushTickerLocked() *time.Ticker {
	if r.stop == nil {
		return nil
	}
	close(r.stop)
	r.stop = nil
	ticker := r.ticker
	r.ticker = nil
	return ticker
}

// stopFlushAndWait stops the ticker then waits for flushLoop. Caller must not hold r.mu.
func (r *Redis) stopFlushAndWait() {
	r.mu.Lock()
	ticker := r.takeFlushTickerLocked()
	r.mu.Unlock()
	if ticker == nil {
		return
	}
	r.wg.Wait()
	ticker.Stop()
}

// flushLoop EVAL-flushes on each tick until stop.
func (r *Redis) flushLoop(ticker *time.Ticker, stop <-chan struct{}) {
	defer r.wg.Done()
	for {
		select {
		case <-ticker.C:
			_ = r.flushPending()
		case <-stop:
			return
		}
	}
}

// flushPending EVAL leak-then-add for every key with local pours.
func (r *Redis) flushPending() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	now := r.now()
	nowMicro := now.UnixMicro()
	nowUnix := now.Unix()
	for redisKey, state := range r.keys {
		if state.localPours > 0 {
			values, err := r.redis.Eval(pourScript, []string{redisKey}, []string{
				strconv.FormatFloat(r.clock.leak, 'f', -1, 64),
				strconv.FormatFloat(r.clock.capacity, 'f', -1, 64),
				strconv.FormatInt(r.clock.ttlSeconds(), 10),
				strconv.FormatInt(nowMicro, 10),
				strconv.FormatFloat(state.localPours, 'f', -1, 64),
			})
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				_, water, _, parseErr := parseEvalReply(values)
				if parseErr != nil {
					if firstErr == nil {
						firstErr = parseErr
					}
				} else {
					state.redisWater = water
					state.lastSync = nowMicro
					state.localPours = 0
				}
			}
		}
		if state.localPours == 0 && nowUnix >= state.expireAt.Unix() {
			delete(r.keys, redisKey)
		}
	}
	return firstErr
}

// parseEvalReply reads allowed, water, and until-not-full microseconds from an EVAL reply.
func parseEvalReply(values [][]byte) (bool, float64, time.Duration, error) {
	if len(values) != 3 {
		return false, 0, 0, errEvalLen
	}
	flag := string(values[0])
	if flag != "0" && flag != "1" {
		return false, 0, 0, errEvalFlag
	}
	water, waterErr := strconv.ParseFloat(string(values[1]), 64)
	if waterErr != nil {
		return false, 0, 0, errEvalWater
	}
	untilMicro, untilErr := strconv.ParseFloat(string(values[2]), 64)
	if untilErr != nil {
		return false, 0, 0, errEvalUntil
	}
	until := time.Duration(0)
	if untilMicro > 0 {
		until = time.Duration(untilMicro * float64(time.Microsecond))
	}
	return flag == "1", water, until, nil
}
