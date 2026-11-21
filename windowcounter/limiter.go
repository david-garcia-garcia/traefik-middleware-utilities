// Package windowcounter is a Yaegi-safe sliding-window hit counter on SimpleRedis.
package windowcounter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// minSyncRate is Kong Advanced's floor for a positive sync interval.
const minSyncRate = 20 * time.Millisecond

// flushBatchKeys is how many window keys one flush EVAL carries. The spec ceiling is 1024 keys per EVAL (as for MSetEX).
const flushBatchKeys = 256

// flushErrorSlotPrefix marks a flush reply slot whose key failed in the script; the rest of the slot is the engine message.
const flushErrorSlotPrefix = "err:"

// flushBatchScript is Kong's INCRBY + EXPIREAT-if-new for every key in KEYS; ARGV holds (delta, expireAt) pairs in KEYS order.
// Each key runs under redis.pcall and gets one reply slot: its INCRBY result, or "err:<message>" when its INCRBY failed, so a
// failing key neither fails nor re-sends the keys the script already applied. A key whose INCRBY applied counts as applied even
// if its EXPIREAT fails, since retrying it would count it twice. KEYS must list every key (Dragonfly); a numeric for over #KEYS,
// no unpack or table.maxn.
const flushBatchScript = `local out = {}
for i = 1, #KEYS do
  local exists = redis.pcall("exists", KEYS[i])
  local value = redis.pcall("incrby", KEYS[i], ARGV[2 * i - 1])
  if type(value) == "table" then
    value = "err:" .. tostring(value.err)
  elseif exists == 0 then
    redis.pcall("expireat", KEYS[i], ARGV[2 * i])
  end
  out[i] = value
end
return out`

// takeExactScript INCR then EXPIRE if the key has no TTL. KEYS must list the key (Dragonfly).
const takeExactScript = `local n = redis.call("INCR", KEYS[1])
if redis.call("PTTL", KEYS[1]) < 0 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return n`

// flushBatchScriptDigest is Redis sha1hex of flushBatchScript, computed once at package init.
var flushBatchScriptDigest = simpleredis.ScriptSHA1Hex(flushBatchScript)

// takeExactScriptDigest is Redis sha1hex of takeExactScript, computed once at package init.
var takeExactScriptDigest = simpleredis.ScriptSHA1Hex(takeExactScript)

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

	lastOutageErr error     // last failed flush or GET; buffered Take/Peek skip Redis for one sync_rate after lastOutageAt
	lastOutageAt  time.Time // when lastOutageErr was stored; Take/Peek retry Redis once sync_rate has passed
}

// windowState is the buffered count for one Redis window key.
type windowState struct {
	redisKnown int64
	localDelta int64
	expireAt   int64
	flushDelta int64 // amount copied for an in-flight EVAL; 0 means none
	// previousGetAt is when buffered Take last GET this key as the previous window; zero means never.
	previousGetAt time.Time
	// unread is true when the buffer was seeded during a Redis outage without a GET (redisKnown is 0,
	// not the shared count). The next Take/Peek that may contact Redis GETs it; a GET or flush clears it.
	unread bool
}

// New builds a limiter on a SimpleRedis from simpleredis.New. Negative syncRate fails. Positive values below 20ms floor to 20ms. Zero is exact (EVAL INCR plus EXPIRE-if-no-TTL every Take, Redis errors on that call). Positive syncRate buffers locally; a Redis outage keeps this instance's limit with a nil error and Redis is retried after one sync_rate.
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

// takeExact EVAL INCR plus EXPIRE-if-no-TTL on the current window, GET previous, then compare the estimate.
func (l *Limiter) takeExact(ctx context.Context, currentKey, previousKey string, ttlSec int64, weight float64, limit int64) (allowed bool, estimated float64, err error) {
	values, err := l.redis.Eval(ctx, takeExactScript, takeExactScriptDigest, []string{currentKey}, []string{
		strconv.FormatInt(ttlSec, 10),
	})
	if err != nil {
		return false, 0, err
	}
	current, convErr := parseEvalInt(values)
	if convErr != nil {
		return false, 0, convErr
	}
	previous, err := l.getCount(ctx, previousKey)
	if err != nil {
		return false, 0, err
	}
	estimated = float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// takeBuffered admits from redis_known + local_delta and leaves Redis to the flush ticker.
// During a Redis outage it keeps this instance's limit and returns a nil error.
func (l *Limiter) takeBuffered(ctx context.Context, currentKey, previousKey string, expireAt int64, weight float64, limit int64) (allowed bool, estimated float64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	currentState := l.windowLocked(ctx, currentKey, expireAt)
	previous := l.takeBufferedPreviousLocked(ctx, previousKey)
	currentState.localDelta++
	current := currentState.redisKnown + currentState.localDelta
	estimated = float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
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

	current := l.peekCountLocked(ctx, currentKey, expireAt)
	previous := l.bufferedCountLocked(ctx, previousKey)
	estimated = float64(current) + float64(previous)*weight
	return estimated <= float64(limit), estimated, nil
}

// peekCountLocked returns redis_known + local_delta without incrementing. GET only on first sight of
// redisKey, or for a buffer seeded during an outage once Redis may be contacted again.
func (l *Limiter) peekCountLocked(ctx context.Context, redisKey string, expireAt int64) int64 {
	state := l.windows[redisKey]
	if state != nil {
		// Already in the buffer: do not GET just because localDelta is 0.
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		if !needsReadLocked(state) {
			return state.redisKnown + state.localDelta
		}
	}
	count, _ := l.readCountLocked(ctx, redisKey, expireAt)
	return count
}

// windowLocked returns the buffer for a Redis key, GET-refreshing redis_known while localDelta is 0.
func (l *Limiter) windowLocked(ctx context.Context, redisKey string, expireAt int64) *windowState {
	state := l.windows[redisKey]
	if state != nil {
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		if state.localDelta != 0 {
			return state
		}
	}
	l.readCountLocked(ctx, redisKey, expireAt)
	return l.windows[redisKey]
}

// takeBufferedPreviousLocked is redis_known + local_delta for previous on Take.
// Other instances may flush into this key after this instance's last flush (Sleep order, or
// their sync_rate tick after the window roll). Take GETs it the first time it is used as previous,
// then at most once per sync_rate, so the refresh cadence matches the current window (one GET per
// flush cycle) instead of one GET per Take. While localDelta is greater than 0 the memory count
// wins: Redis does not hold this instance's unflushed delta. The GET drops l.mu like windowLocked.
func (l *Limiter) takeBufferedPreviousLocked(ctx context.Context, redisKey string) int64 {
	now := l.now()
	state := l.windows[redisKey]
	if state != nil && !needsReadLocked(state) {
		refreshed := !state.previousGetAt.IsZero() && now.Sub(state.previousGetAt) < l.syncRate
		if state.localDelta != 0 || refreshed {
			return state.redisKnown + state.localDelta
		}
	}
	count, read := l.readCountLocked(ctx, redisKey, 0)
	if read {
		l.windows[redisKey].previousGetAt = now
	}
	return count
}

// bufferedCountLocked is redis_known + local_delta, GET-seeding a key the limiter has not read from Redis.
func (l *Limiter) bufferedCountLocked(ctx context.Context, redisKey string) int64 {
	state := l.windows[redisKey]
	if state != nil && !needsReadLocked(state) {
		return state.redisKnown + state.localDelta
	}
	count, _ := l.readCountLocked(ctx, redisKey, 0)
	return count
}

// needsReadLocked is true for a buffer seeded during an outage that holds no local or in-flight delta.
// A buffer with a delta gets the shared count back from its flush EVAL instead.
func needsReadLocked(state *windowState) bool {
	return state.unread && state.localDelta == 0 && state.flushDelta == 0
}

// readCountLocked GETs redisKey into the buffer and returns redis_known + local_delta.
// During a stored outage (one sync_rate after the last failure) it skips Redis. A failed GET stores
// the outage. Either way the key is seeded at zero (unread) so Take/Peek keep this instance's limit.
// read is true only when the GET succeeded. Caller holds l.mu; the GET drops it.
func (l *Limiter) readCountLocked(ctx context.Context, redisKey string, expireAt int64) (count int64, read bool) {
	if l.skipRedisContactLocked() {
		state := l.seedUnreadWindowLocked(redisKey, expireAt)
		return state.redisKnown + state.localDelta, false
	}
	known, err := l.getCountUnlocked(ctx, redisKey)
	if err != nil {
		l.rememberOutageLocked(err)
		state := l.seedUnreadWindowLocked(redisKey, expireAt)
		return state.redisKnown + state.localDelta, false
	}
	l.lastOutageErr = nil
	state := l.applyGetLocked(redisKey, known, expireAt)
	return state.redisKnown + state.localDelta, true
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
		state.unread = false
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

// pendingFlush is one window's EVAL snapshot copied under l.mu.
type pendingFlush struct {
	redisKey string
	delta    int64
	expireAt int64
}

// flushPendingLocked copies in-flight deltas, EVALs them in batches of flushBatchKeys without l.mu, then applies each key's
// outcome. Caller holds l.mu. A second flusher skips a key whose flushDelta is set, so a later batch's keys stay in flight
// until their EVAL returns.
func (l *Limiter) flushPendingLocked(ctx context.Context) error {
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
	// EVAL each batch unlocked, then apply each key's slot: applied keys leave pending, failed keys keep localDelta.
	for batchStart := 0; batchStart < len(pending); batchStart += flushBatchKeys {
		batchEnd := batchStart + flushBatchKeys
		if batchEnd > len(pending) {
			batchEnd = len(pending)
		}
		batch := pending[batchStart:batchEnd]
		values, err := l.flushBatchEvalUnlocked(ctx, batch)
		// A reply without one slot per key cannot be matched to keys, so every key of this batch fails.
		if err == nil && len(values) != len(batch) {
			err = errors.New(simpleredis.RedisIssue)
		}
		for i, job := range batch {
			slotErr := err
			n := int64(0)
			if slotErr == nil {
				n, slotErr = parseFlushSlot(values[i])
			}
			if slotErr != nil && firstErr == nil {
				firstErr = slotErr
			}
			l.applyFlushOutcomeLocked(job, n, slotErr)
		}
	}
	// Drop windows whose TTL has passed and that have no local or in-flight delta.
	for redisKey, state := range l.windows {
		if state.localDelta == 0 && state.flushDelta == 0 && (state.expireAt == 0 || nowUnix >= state.expireAt) {
			delete(l.windows, redisKey)
		}
	}
	if firstErr != nil {
		l.lastOutageErr = firstErr
		l.lastOutageAt = l.now()
	}
	return firstErr
}

// applyFlushOutcomeLocked applies one key's flush result. Applied: redisKnown = n and localDelta drops by the amount sent,
// keeping hits added while the EVAL was in flight. Failed: localDelta is kept for the next flush. Caller holds l.mu.
func (l *Limiter) applyFlushOutcomeLocked(job pendingFlush, n int64, slotErr error) {
	state := l.windows[job.redisKey]
	if state == nil {
		return
	}
	state.flushDelta = 0
	if slotErr != nil {
		return
	}
	state.redisKnown = n
	state.unread = false
	state.localDelta -= job.delta
	l.lastOutageErr = nil
}

// flushBatchEvalUnlocked drops l.mu for one batch flush EVAL then re-locks. Caller holds l.mu.
func (l *Limiter) flushBatchEvalUnlocked(ctx context.Context, batch []pendingFlush) ([][]byte, error) {
	keys := make([]string, len(batch))
	args := make([]string, 0, 2*len(batch))
	for i, job := range batch {
		keys[i] = job.redisKey
		args = append(args, strconv.FormatInt(job.delta, 10), strconv.FormatInt(job.expireAt, 10))
	}
	l.mu.Unlock()
	defer l.mu.Lock()
	return l.redis.Eval(ctx, flushBatchScript, flushBatchScriptDigest, keys, args)
}

// parseFlushSlot reads one key's slot from a flush batch reply: the INCRBY integer, or the engine error the script reported
// for that key. Any other slot wraps the conversion failure under redis:issue?.
func parseFlushSlot(slot []byte) (int64, error) {
	text := string(slot)
	if strings.HasPrefix(text, flushErrorSlotPrefix) {
		return 0, errors.New(strings.TrimPrefix(text, flushErrorSlotPrefix))
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", simpleredis.RedisIssue, err)
	}
	return n, nil
}

// skipRedisContactLocked is true for one sync_rate after a failed flush or GET. After that the next
// Take/Peek GET probes Redis again, so a node with no pending delta (Peek-only) still recovers.
// A successful GET or flush clears the outage.
func (l *Limiter) skipRedisContactLocked() bool {
	return l.lastOutageErr != nil && l.now().Sub(l.lastOutageAt) < l.syncRate
}

// rememberOutageLocked stores err so buffered Take/Peek skip Redis for one sync_rate instead of retrying GET.
func (l *Limiter) rememberOutageLocked(err error) {
	l.lastOutageErr = err
	l.lastOutageAt = l.now()
}

// seedUnreadWindowLocked returns the buffer for redisKey, creating a zero unread one if missing.
// It never replaces a buffer another Take created while l.mu was dropped for a GET.
func (l *Limiter) seedUnreadWindowLocked(redisKey string, expireAt int64) *windowState {
	state := l.windows[redisKey]
	if state != nil {
		if expireAt > state.expireAt {
			state.expireAt = expireAt
		}
		return state
	}
	state = &windowState{expireAt: expireAt, unread: true}
	l.windows[redisKey] = state
	return state
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
