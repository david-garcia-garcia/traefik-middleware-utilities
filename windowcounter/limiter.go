// Package windowcounter is a Yaegi-safe sliding-window hit counter on SimpleRedis.
package windowcounter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// minSyncRate is Kong Advanced's floor for a positive sync interval.
const minSyncRate = 20 * time.Millisecond

// flushScript is Kong's INCRBY + EXPIREAT-if-new snippet. KEYS must list the key (Dragonfly).
const flushScript = `local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value`

// flushScriptDigest is Redis sha1hex of flushScript, computed once at package init.
var flushScriptDigest = simpleredis.ScriptSHA1Hex(flushScript)

// Limiter admits hits on opaque keys against a sliding Redis window. Local memory is only the sync_rate buffer.
type Limiter struct {
	redis    *simpleredis.SimpleRedis
	syncRate time.Duration
	now      func() time.Time

	mu      sync.Mutex
	windows map[string]*windowState
	closed  bool
	ticker  *time.Ticker
	stop    chan struct{}
	wg      sync.WaitGroup

	lastFlushErr  error     // last failed flush, returned by buffered Take/Peek
	flushFailedAt time.Time // when lastFlushErr was stored
	lastRedisOK   time.Time // last successful Redis GET or flush
}

// windowState is the buffered count for one Redis window key.
type windowState struct {
	redisKnown int64
	localDelta int64
	expireAt   int64
}

// New builds a limiter on a SimpleRedis from simpleredis.New. Negative syncRate fails. Positive values below 20ms floor to 20ms. Zero is exact (INCR every Take, Redis errors on that call). Positive syncRate buffers locally and returns a retained flush error (or a probe after one missed sync_rate) instead of a silent nil.
func New(redis *simpleredis.SimpleRedis, syncRate time.Duration) (*Limiter, error) {
	if redis == nil {
		return nil, errors.New("windowcounter: redis is required")
	}
	if syncRate < 0 {
		return nil, errors.New("windowcounter: sync_rate must not be negative")
	}
	if syncRate > 0 && syncRate < minSyncRate {
		syncRate = minSyncRate
	}
	limiter := &Limiter{
		redis:    redis,
		syncRate: syncRate,
		now:      time.Now,
		windows:  map[string]*windowState{},
	}
	if syncRate > 0 {
		limiter.startFlushLocked()
	}
	return limiter, nil
}

// SetNowForTest replaces the clock. Production callers must not use this.
func (l *Limiter) SetNowForTest(now func() time.Time) {
	if now == nil {
		l.now = time.Now
		return
	}
	l.now = now
}

// slidingWindow is the Redis keys, previous-window weight, and TTL for one Take or Peek at now.
type slidingWindow struct {
	currentKey  string
	previousKey string
	weight      float64
	ttlSec      int64
	expireAt    int64
}

// slidingAt builds the current and previous window keys and the previous-window weight.
func (l *Limiter) slidingAt(key string, window time.Duration) (slidingWindow, error) {
	// Reject sub-second and fractional-second windows; do not truncate into buckets.
	if window < time.Second {
		return slidingWindow{}, errors.New("windowcounter: window must be at least one second")
	}
	if window%time.Second != 0 {
		return slidingWindow{}, errors.New("windowcounter: window must be a whole number of seconds")
	}
	windowSec := int64(window / time.Second)
	// Keys and previous-window weight at the caller's clock (whole seconds).
	now := l.now()
	windowStart := now.Unix() / windowSec * windowSec
	previousStart := windowStart - windowSec
	elapsed := time.Duration(now.Unix()-windowStart) * time.Second
	weight := 1 - float64(elapsed)/(float64(windowSec)*float64(time.Second))
	if weight < 0 {
		weight = 0
	}
	ttlSec := 2 * windowSec
	return slidingWindow{
		currentKey:  redisWindowKey(key, windowStart),
		previousKey: redisWindowKey(key, previousStart),
		weight:      weight,
		ttlSec:      ttlSec,
		expireAt:    windowStart + ttlSec,
	}, nil
}

// Take counts one hit on key against limit and window, then returns whether it is allowed and the sliding estimate.
func (l *Limiter) Take(ctx context.Context, key string, limit int64, window time.Duration) (bool, float64, error) {
	if err := ctx.Err(); err != nil {
		return false, 0, err
	}
	sliding, err := l.slidingAt(key, window)
	if err != nil {
		return false, 0, err
	}
	if l.syncRate == 0 {
		return l.takeExact(ctx, sliding.currentKey, sliding.previousKey, sliding.ttlSec, sliding.weight, limit)
	}
	return l.takeBuffered(ctx, sliding.currentKey, sliding.previousKey, sliding.expireAt, sliding.weight, limit)
}

// Peek returns whether a hit would be allowed and the sliding estimate without incrementing.
func (l *Limiter) Peek(ctx context.Context, key string, limit int64, window time.Duration) (bool, float64, error) {
	if err := ctx.Err(); err != nil {
		return false, 0, err
	}
	sliding, err := l.slidingAt(key, window)
	if err != nil {
		return false, 0, err
	}
	if l.syncRate == 0 {
		return l.peekExact(ctx, sliding.currentKey, sliding.previousKey, sliding.weight, limit)
	}
	return l.peekBuffered(ctx, sliding.currentKey, sliding.previousKey, sliding.expireAt, sliding.weight, limit)
}

// Allow is an alias for Take for callers who prefer Allow.
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, float64, error) {
	return l.Take(ctx, key, limit, window)
}

// takeExact INCR the current window, EXPIRE on first hit, GET previous, then compare the estimate.
func (l *Limiter) takeExact(ctx context.Context, currentKey, previousKey string, ttlSec int64, weight float64, limit int64) (bool, float64, error) {
	current, err := l.redis.Incr(ctx, currentKey)
	if err != nil {
		return false, 0, err
	}
	if current == 1 {
		if expireErr := l.redis.Expire(ctx, currentKey, ttlSec); expireErr != nil {
			return false, 0, expireErr
		}
	}
	previous, err := l.getCount(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	estimated := float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// takeBuffered admits from redis_known + local_delta and leaves Redis to the flush ticker.
func (l *Limiter) takeBuffered(ctx context.Context, currentKey, previousKey string, expireAt int64, weight float64, limit int64) (bool, float64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentState, err := l.windowLocked(ctx, currentKey, expireAt)
	if err != nil {
		return false, 0, err
	}
	previous, err := l.takeBufferedPreviousLocked(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	currentState.localDelta++
	current := currentState.redisKnown + currentState.localDelta
	estimated := float64(current) + float64(previous)*weight
	// Local admit stays on the return even when a flush/probe error is set.
	return estimated <= float64(limit), estimated, l.bufferedOutageErrorLocked(ctx)
}

// peekExact GETs current and previous without INCR or EXPIRE, then compares the estimate.
func (l *Limiter) peekExact(ctx context.Context, currentKey, previousKey string, weight float64, limit int64) (bool, float64, error) {
	current, err := l.getCount(ctx, currentKey)
	if err != nil {
		return false, 0, err
	}
	previous, err := l.getCount(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	estimated := float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// peekBuffered reads redis_known + local_delta under the Take lock without incrementing.
func (l *Limiter) peekBuffered(ctx context.Context, currentKey, previousKey string, expireAt int64, weight float64, limit int64) (bool, float64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	current, err := l.peekCountLocked(ctx, currentKey, expireAt)
	if err != nil {
		return false, 0, err
	}
	previous, err := l.bufferedCountLocked(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	estimated := float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, l.bufferedOutageErrorLocked(ctx)
}

// peekCountLocked returns redis_known + local_delta without incrementing. GET only on first sight of redisKey.
func (l *Limiter) peekCountLocked(ctx context.Context, redisKey string, expireAt int64) (int64, error) {
	state := l.windows[redisKey]
	if state != nil {
		// Already in the buffer: do not GET just because localDelta is 0.
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		return state.redisKnown + state.localDelta, nil
	}
	// Seed redis_known from Redis on first sight of this window key.
	known, err := l.getCount(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	l.lastRedisOK = l.now()
	l.windows[redisKey] = &windowState{redisKnown: known, expireAt: expireAt}
	return known, nil
}

// windowLocked returns the buffer for a Redis key, seeding redis_known from GET on first sight.
func (l *Limiter) windowLocked(ctx context.Context, redisKey string, expireAt int64) (*windowState, error) {
	state := l.windows[redisKey]
	if state != nil {
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		if state.localDelta == 0 {
			known, err := l.getCount(ctx, redisKey)
			if err != nil {
				return nil, err
			}
			l.lastRedisOK = l.now()
			state.redisKnown = known
		}
		return state, nil
	}
	known, err := l.getCount(ctx, redisKey)
	if err != nil {
		return nil, err
	}
	l.lastRedisOK = l.now()
	state = &windowState{redisKnown: known, expireAt: expireAt}
	l.windows[redisKey] = state
	return state, nil
}

// takeBufferedPreviousLocked is redis_known + local_delta for previous on Take.
// GET when this instance already counted that key (expireAt set) and localDelta is 0,
// so a later flush from another instance is visible after Sleep.
// Keep memory when localDelta is greater than 0, or when the key was only GET-seeded as previous
// (expireAt 0): GET every Take would GET Redis on every buffered call.
// Do not write current-window expireAt onto this key.
func (l *Limiter) takeBufferedPreviousLocked(ctx context.Context, redisKey string) (int64, error) {
	state := l.windows[redisKey]
	if state != nil {
		if state.localDelta == 0 && state.expireAt > 0 {
			// Shared Redis may have moved since this node's last flush of this key.
			known, err := l.getCount(ctx, redisKey)
			if err != nil {
				return 0, err
			}
			l.lastRedisOK = l.now()
			state.redisKnown = known
		}
		return state.redisKnown + state.localDelta, nil
	}
	// Seed redis_known from Redis on first sight of this previous-window key.
	known, err := l.getCount(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	l.lastRedisOK = l.now()
	l.windows[redisKey] = &windowState{redisKnown: known}
	return known, nil
}

// bufferedCountLocked is redis_known + local_delta, GET-seeding a key the limiter has not seen.
func (l *Limiter) bufferedCountLocked(ctx context.Context, redisKey string) (int64, error) {
	state := l.windows[redisKey]
	if state != nil {
		return state.redisKnown + state.localDelta, nil
	}
	known, err := l.getCount(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	l.lastRedisOK = l.now()
	l.windows[redisKey] = &windowState{redisKnown: known}
	return known, nil
}

// getCount reads a Redis integer. A miss is zero.
func (l *Limiter) getCount(ctx context.Context, redisKey string) (int64, error) {
	raw, err := l.redis.Get(ctx, redisKey)
	return countFromRedisGet(raw, err)
}

// countFromRedisGet turns a GET payload into a window count. A miss, including a wrapped miss, is zero.
func countFromRedisGet(raw []byte, err error) (int64, error) {
	if err != nil {
		if simpleredis.IsMiss(err) {
			return 0, nil
		}
		return 0, err
	}
	n, convErr := strconv.ParseInt(string(raw), 10, 64)
	if convErr != nil {
		return 0, errors.New(simpleredis.RedisIssue)
	}
	return n, nil
}

// Sleep flushes pending deltas then stops the flush ticker. Exact mode is a no-op besides stopping leftover work.
func (l *Limiter) Sleep() {
	_ = l.flushPending(context.Background())
	l.stopFlushAndWait()
}

// Wake starts the flush ticker when sync_rate is positive and the limiter is not closed.
func (l *Limiter) Wake() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.syncRate == 0 || l.stop != nil {
		return
	}
	l.startFlushLocked()
}

// Close stops the ticker after Sleep and refuses to start another. It does not close the injected SimpleRedis.
func (l *Limiter) Close() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	l.mu.Unlock()
	_ = l.flushPending(context.Background())
	l.stopFlushAndWait()
}

// startFlushLocked starts the ticker goroutine. Caller holds l.mu. NewTicker is not time.Tick.
func (l *Limiter) startFlushLocked() {
	l.stop = make(chan struct{})
	l.ticker = time.NewTicker(l.syncRate)
	l.wg.Add(1)
	go l.flushLoop(l.ticker, l.stop)
}

// takeFlushTickerLocked closes the stop channel and detaches the ticker. Caller holds l.mu. Does not wait.
func (l *Limiter) takeFlushTickerLocked() *time.Ticker {
	if l.stop == nil {
		return nil
	}
	close(l.stop)
	l.stop = nil
	ticker := l.ticker
	l.ticker = nil
	return ticker
}

// stopFlushAndWait stops the ticker then waits for flushLoop. Caller must not hold l.mu.
func (l *Limiter) stopFlushAndWait() {
	l.mu.Lock()
	ticker := l.takeFlushTickerLocked()
	l.mu.Unlock()
	if ticker == nil {
		return
	}
	l.wg.Wait()
	ticker.Stop()
}

// flushLoop EVAL-flushes on each tick until stop.
func (l *Limiter) flushLoop(ticker *time.Ticker, stop <-chan struct{}) {
	defer l.wg.Done()
	for {
		select {
		case <-ticker.C:
			_ = l.flushPending(context.Background())
		case <-stop:
			return
		}
	}
}

// flushPending EVAL INCRBY + EXPIREAT-if-new for every window with a local delta.
func (l *Limiter) flushPending(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushPendingLocked(ctx)
}

// flushPendingLocked is flushPending while the caller already holds l.mu.
func (l *Limiter) flushPendingLocked(ctx context.Context) error {
	var firstErr error
	nowUnix := l.now().Unix()
	for redisKey, state := range l.windows {
		if state.localDelta > 0 {
			delta := state.localDelta
			expireAt := state.expireAt
			values, err := l.redis.Eval(ctx, flushScript, flushScriptDigest, []string{redisKey}, []string{
				strconv.FormatInt(delta, 10),
				strconv.FormatInt(expireAt, 10),
			})
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				n, convErr := parseEvalInt(values)
				if convErr != nil {
					if firstErr == nil {
						firstErr = convErr
					}
				} else {
					state.redisKnown = n
					state.localDelta = 0
					l.lastRedisOK = l.now()
					l.lastFlushErr = nil
				}
			}
		}
		if state.localDelta == 0 && (state.expireAt == 0 || nowUnix >= state.expireAt) {
			delete(l.windows, redisKey)
		}
	}
	if firstErr != nil {
		l.lastFlushErr = firstErr
		l.flushFailedAt = l.now()
	}
	return firstErr
}

// bufferedOutageErrorLocked returns a retained flush error, or probes Redis after one missed sync_rate.
func (l *Limiter) bufferedOutageErrorLocked(ctx context.Context) error {
	if l.lastFlushErr != nil {
		return l.lastFlushErr
	}
	contactedWithinSyncRate := !l.lastRedisOK.IsZero() && l.now().Sub(l.lastRedisOK) < l.syncRate
	if contactedWithinSyncRate {
		return nil
	}
	// Probe with the pending EVAL flush first so a hot key does not GET every Take.
	if err := l.flushPendingLocked(ctx); err != nil {
		return err
	}
	if l.lastFlushErr != nil {
		return l.lastFlushErr
	}
	contactedWithinSyncRate = !l.lastRedisOK.IsZero() && l.now().Sub(l.lastRedisOK) < l.syncRate
	if contactedWithinSyncRate {
		return nil
	}
	// Nothing flushed; GET one buffered key to observe an outage Peek would otherwise miss.
	for redisKey := range l.windows {
		_, err := l.getCount(ctx, redisKey)
		if err != nil {
			l.lastFlushErr = err
			l.flushFailedAt = l.now()
			return err
		}
		l.lastRedisOK = l.now()
		return nil
	}
	return nil
}

// parseEvalInt reads one integer from an EVAL reply.
func parseEvalInt(values [][]byte) (int64, error) {
	if len(values) != 1 {
		return 0, errors.New(simpleredis.RedisIssue)
	}
	n, err := strconv.ParseInt(string(values[0]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", simpleredis.RedisIssue, err)
	}
	return n, nil
}

// redisWindowKey is {opaqueKey}:{windowStartUnix}.
func redisWindowKey(opaqueKey string, windowStart int64) string {
	return opaqueKey + ":" + strconv.FormatInt(windowStart, 10)
}
