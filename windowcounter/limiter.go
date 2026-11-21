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

	mu         sync.Mutex
	windows    map[string]*windowState
	closed     bool
	stopping   bool // true while stopFlushAndWait has detached the timer and is waiting for an in-flight tick
	flushTimer *time.Timer
	flushBusy  sync.WaitGroup // 1 while a timer is armed or flushAfterTick is running

	lastFlushErr  error     // last failed flush, returned by buffered Take/Peek
	flushFailedAt time.Time // when lastFlushErr was stored
	lastRedisOK   time.Time // last successful Redis GET or flush
}

// windowState is the buffered count for one Redis window key.
type windowState struct {
	redisKnown int64
	localDelta int64
	expireAt   int64
	flushDelta int64 // amount copied for an in-flight EVAL; 0 means none
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
func (l *Limiter) Take(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, estimated float64, err error) {
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

// Peek reports whether already-used occupancy is at or under limit, and the sliding estimate, without recording a hit.
// allowed is occupancy, not whether the next Take would admit.
func (l *Limiter) Peek(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, estimated float64, err error) {
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
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (allowed bool, estimated float64, err error) {
	return l.Take(ctx, key, limit, window)
}

// takeExact INCR the current window, EXPIRE on first hit, GET previous, then compare the estimate.
func (l *Limiter) takeExact(ctx context.Context, currentKey, previousKey string, ttlSec int64, weight float64, limit int64) (allowed bool, estimated float64, err error) {
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
	estimated = float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// takeBuffered admits from redis_known + local_delta and leaves Redis to the flush ticker.
func (l *Limiter) takeBuffered(ctx context.Context, currentKey, previousKey string, expireAt int64, weight float64, limit int64) (allowed bool, estimated float64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentState, err := l.windowLocked(ctx, currentKey, expireAt)
	if err != nil {
		return false, 0, err
	}
	previous, err := l.bufferedCountLocked(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	currentState.localDelta++
	current := currentState.redisKnown + currentState.localDelta
	estimated = float64(current) + float64(previous)*weight
	// Local admit stays on the return even when a flush/probe error is set.
	return estimated <= float64(limit), estimated, l.bufferedOutageErrorLocked(ctx)
}

// peekExact GETs current and previous without INCR or EXPIRE, then compares the estimate.
func (l *Limiter) peekExact(ctx context.Context, currentKey, previousKey string, weight float64, limit int64) (allowed bool, estimated float64, err error) {
	current, err := l.getCount(ctx, currentKey)
	if err != nil {
		return false, 0, err
	}
	previous, err := l.getCount(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	estimated = float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// peekBuffered reads redis_known + local_delta under the Take lock without incrementing.
func (l *Limiter) peekBuffered(ctx context.Context, currentKey, previousKey string, expireAt int64, weight float64, limit int64) (allowed bool, estimated float64, err error) {
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
	estimated = float64(current) + float64(previous)*weight
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
	known, err := l.getCountUnlocked(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	l.lastRedisOK = l.now()
	state = l.applyGetLocked(redisKey, known, expireAt)
	return state.redisKnown + state.localDelta, nil
}

// windowLocked returns the buffer for a Redis key, seeding redis_known from GET on first sight.
func (l *Limiter) windowLocked(ctx context.Context, redisKey string, expireAt int64) (*windowState, error) {
	state := l.windows[redisKey]
	if state != nil {
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		if state.localDelta != 0 {
			return state, nil
		}
	}
	known, err := l.getCountUnlocked(ctx, redisKey)
	if err != nil {
		return nil, err
	}
	l.lastRedisOK = l.now()
	return l.applyGetLocked(redisKey, known, expireAt), nil
}

// bufferedCountLocked is redis_known + local_delta, GET-seeding a key the limiter has not seen.
func (l *Limiter) bufferedCountLocked(ctx context.Context, redisKey string) (int64, error) {
	state := l.windows[redisKey]
	if state != nil {
		return state.redisKnown + state.localDelta, nil
	}
	known, err := l.getCountUnlocked(ctx, redisKey)
	if err != nil {
		return 0, err
	}
	l.lastRedisOK = l.now()
	state = l.applyGetLocked(redisKey, known, 0)
	return state.redisKnown + state.localDelta, nil
}

// getCountUnlocked drops l.mu for Redis GET then re-locks. Caller holds l.mu.
func (l *Limiter) getCountUnlocked(ctx context.Context, redisKey string) (int64, error) {
	l.mu.Unlock()
	defer l.mu.Lock()
	return l.getCount(ctx, redisKey)
}

// applyGetLocked stores a GET count. Caller holds l.mu. A concurrent Take's localDelta is kept.
func (l *Limiter) applyGetLocked(redisKey string, known, expireAt int64) *windowState {
	state := l.windows[redisKey]
	if state == nil {
		state = &windowState{redisKnown: known, expireAt: expireAt}
		l.windows[redisKey] = state
		return state
	}
	if expireAt > state.expireAt {
		state.expireAt = expireAt
	}
	if state.localDelta == 0 {
		state.redisKnown = known
	}
	return state
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

// Wake starts the flush timer when sync_rate is positive, the limiter is not closed, and Sleep or Close is not waiting for an in-flight tick.
func (l *Limiter) Wake() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.syncRate == 0 || l.flushTimer != nil || l.stopping {
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

// startFlushLocked arms a stdlib AfterFunc timer. Caller holds l.mu except during New.
//
// Why AfterFunc, not go + select on ticker.C and stop: this package is interpreted under Yaegi
// (Traefik plugins and TestYaegi_*). Yaegi v0.16.1's interp._select can miss close(stop) when
// interpreted code selects on a channel from a goroutine started as go method(...). CI Go E2E
// Dragonfly hung 300s on TestYaegiLive_RedisAndDragonfly/dragonfly/bufferedTwoClients that way
// (Eval parked in WaitGroup.Wait; flushLoop parked in interp._select). time.AfterFunc is the
// real stdlib timer (Yaegi maps the symbol to time.AfterFunc), so the waiter is not interpreted
// select. Do not use time.Tick.
func (l *Limiter) startFlushLocked() {
	if l.flushTimer != nil {
		return
	}
	l.flushBusy.Add(1)
	l.flushTimer = time.AfterFunc(l.syncRate, l.flushAfterTick)
}

// flushAfterTick EVAL-flushes then re-arms, unless Sleep or Close has stopped the timer.
func (l *Limiter) flushAfterTick() {
	_ = l.flushPending(context.Background())
	l.mu.Lock()
	if l.closed || l.stopping || l.flushTimer == nil {
		l.mu.Unlock()
		l.flushBusy.Done()
		return
	}
	l.flushTimer = time.AfterFunc(l.syncRate, l.flushAfterTick)
	l.mu.Unlock()
}

// stopFlushAndWait stops the flush timer then waits for an in-flight tick. Caller must not hold l.mu.
func (l *Limiter) stopFlushAndWait() {
	l.mu.Lock()
	l.stopping = true
	timer := l.flushTimer
	l.flushTimer = nil
	if timer == nil {
		l.stopping = false
		l.mu.Unlock()
		return
	}
	l.mu.Unlock()
	if timer.Stop() {
		l.flushBusy.Done()
	}
	l.flushBusy.Wait()
	l.mu.Lock()
	l.stopping = false
	l.mu.Unlock()
}

// flushPending EVAL INCRBY + EXPIREAT-if-new for every window with a local delta.
func (l *Limiter) flushPending(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flushPendingLocked(ctx)
}

// flushPendingLocked copies in-flight deltas, EVAL without l.mu, then subtracts flushed amount. Caller holds l.mu. A second flusher skips a key whose flushDelta is set.
func (l *Limiter) flushPendingLocked(ctx context.Context) error {
	// pendingFlush is one window's EVAL snapshot copied under l.mu.
	type pendingFlush struct {
		redisKey string
		delta    int64
		expireAt int64
	}
	// Copy (key, delta, expireAt) and mark in-flight so a second flusher skips this snapshot.
	var pending []pendingFlush
	nowUnix := l.now().Unix()
	for redisKey, state := range l.windows {
		if state.localDelta > 0 && state.flushDelta == 0 {
			state.flushDelta = state.localDelta
			pending = append(pending, pendingFlush{
				redisKey: redisKey,
				delta:    state.flushDelta,
				expireAt: state.expireAt,
			})
		}
	}
	var firstErr error
	// EVAL each snapshot unlocked; on success redisKnown = n and localDelta -= flushedDelta.
	for _, job := range pending {
		values, err := l.flushEvalUnlocked(ctx, job.redisKey, job.delta, job.expireAt)
		state := l.windows[job.redisKey]
		if err != nil {
			if state != nil {
				state.flushDelta = 0
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n, convErr := parseEvalInt(values)
		if convErr != nil {
			if state != nil {
				state.flushDelta = 0
			}
			if firstErr == nil {
				firstErr = convErr
			}
			continue
		}
		if state == nil {
			continue
		}
		state.redisKnown = n
		state.localDelta -= job.delta
		state.flushDelta = 0
		l.lastRedisOK = l.now()
		l.lastFlushErr = nil
	}
	// Drop windows whose TTL has passed and that have no local or in-flight delta.
	for redisKey, state := range l.windows {
		if state.localDelta == 0 && state.flushDelta == 0 && (state.expireAt == 0 || nowUnix >= state.expireAt) {
			delete(l.windows, redisKey)
		}
	}
	if firstErr != nil {
		l.lastFlushErr = firstErr
		l.flushFailedAt = l.now()
	}
	return firstErr
}

// flushEvalUnlocked drops l.mu for flush EVAL then re-locks. Caller holds l.mu.
func (l *Limiter) flushEvalUnlocked(ctx context.Context, redisKey string, delta, expireAt int64) ([][]byte, error) {
	l.mu.Unlock()
	defer l.mu.Lock()
	return l.redis.Eval(ctx, flushScript, flushScriptDigest, []string{redisKey}, []string{
		strconv.FormatInt(delta, 10),
		strconv.FormatInt(expireAt, 10),
	})
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
	var probeKey string
	for redisKey := range l.windows {
		probeKey = redisKey
		break
	}
	if probeKey == "" {
		return nil
	}
	_, err := l.getCountUnlocked(ctx, probeKey)
	if err != nil {
		l.lastFlushErr = err
		l.flushFailedAt = l.now()
		return err
	}
	l.lastRedisOK = l.now()
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
