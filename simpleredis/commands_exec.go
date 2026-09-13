package simpleredis

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"
)

// nopCancel is bindCommandDeadline's cancel when the parent already expires first.
var nopCancel context.CancelFunc = func() {}

// exec borrows a connection, runs one RESP command, and retries retryable failures up to MaxRetries.
// The library overall deadline is bound onto ctx (stdlib Dialer/Client shape). Caller cancel stays ctx.Err(); library expiry is redis:timeout. INCR/INCRBY/EVAL can double-apply when a reply is lost and the command is sent again; that is accepted.
func (sr *SimpleRedis) exec(ctx context.Context, args ...[]byte) ([][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxRetries, minBackoff, maxBackoff := retryLimits(sr.maxRetries, sr.minRetryBackoff, sr.maxRetryBackoff)
	ctx, cancel, libraryOwnsDeadline := sr.bindCommandDeadline(ctx, maxRetries)
	defer cancel()

	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := contextStop(ctx); err != nil {
			return nil, libraryTimeout(err, libraryOwnsDeadline)
		}
		if attempt > 0 {
			if err := waitUntil(ctx, retryBackoff(attempt, minBackoff, maxBackoff)); err != nil {
				return nil, libraryTimeout(err, libraryOwnsDeadline)
			}
		}
		conn, err := sr.borrow(ctx)
		if err != nil {
			if sr.isClosed() || !shouldRetry(err) {
				return nil, libraryTimeout(err, libraryOwnsDeadline)
			}
			last = err
			continue
		}
		values, err := sr.runOnConn(ctx, conn, args)
		if err == nil {
			return values, nil
		}
		if !shouldRetry(err) {
			return values, libraryTimeout(err, libraryOwnsDeadline)
		}
		last = err
	}
	if last != nil {
		return nil, libraryTimeout(last, libraryOwnsDeadline)
	}
	return nil, errTimeout
}

// runOnConn runs one command on conn and always releases it, including when do panics.
// reusable starts false because a panic proves nothing about the socket's protocol position:
// the release then closes the socket and returns the in-use turn instead of leaking both.
func (sr *SimpleRedis) runOnConn(ctx context.Context, conn *pooledConn, args [][]byte) (values [][]byte, err error) {
	reusable := false
	defer func() { sr.release(conn, reusable) }()
	values, reusable, err = sr.do(ctx, conn, args)
	if stop := contextStop(ctx); stop != nil {
		reusable = false
		return nil, stop
	}
	return values, err
}

// bindCommandDeadline wraps ctx with (maxRetries+1)*(DialTimeout+IOTimeout) when that instant is sooner than the parent.
// Same shape as net.Dialer / http.Client: one child context, not a parallel time.Time next to ctx.
// libraryOwnsDeadline is true when DeadlineExceeded on the returned ctx is the library budget (maps to redis:timeout).
func (sr *SimpleRedis) bindCommandDeadline(ctx context.Context, maxRetries int) (context.Context, context.CancelFunc, bool) {
	libraryDeadline := time.Now().Add(time.Duration(maxRetries+1) * (sr.DialTimeout() + sr.IOTimeout()))
	if parent, ok := ctx.Deadline(); ok && !parent.After(libraryDeadline) {
		return ctx, nopCancel, false
	}
	ctx, cancel := context.WithDeadline(ctx, libraryDeadline)
	return ctx, cancel, true
}

// libraryTimeout maps a library-owned context deadline to redis:timeout. Caller cancel and a sooner caller deadline stay ctx.Err().
func libraryTimeout(err error, libraryOwnsDeadline bool) error {
	if libraryOwnsDeadline && errors.Is(err, context.DeadlineExceeded) {
		return errTimeout
	}
	return err
}

// contextStop is ctx.Err(), or DeadlineExceeded when the deadline time has passed but Done has not closed yet.
// The timer and the socket SetDeadline are independent clocks; Done can lag the wall clock by a few ms.
func contextStop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

// clampTimeout is limit, or time left on ctx when that is shorter.
func clampTimeout(ctx context.Context, limit time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return limit
	}
	remaining := time.Until(deadline)
	if remaining < limit {
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return limit
}

// waitUntil waits delay or until ctx fires. Library expiry is mapped at exec.
func waitUntil(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return contextStop(ctx)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return contextStop(ctx)
	}
}

// retryLimits maps 0/-1 sentinels to go-redis Options defaults without mutating the exported fields.
func retryLimits(maxRetries int, minBackoff, maxBackoff time.Duration) (int, time.Duration, time.Duration) {
	if maxRetries == -1 {
		maxRetries = 0
	} else if maxRetries == 0 {
		maxRetries = 3
	}
	if minBackoff == -1 {
		minBackoff = 0
	} else if minBackoff == 0 {
		minBackoff = 8 * time.Millisecond
	}
	if maxBackoff == -1 {
		maxBackoff = 0
	} else if maxBackoff == 0 {
		maxBackoff = 512 * time.Millisecond
	}
	return maxRetries, minBackoff, maxBackoff
}

// retryBackoff is go-redis internal.RetryBackoff: exponential from minBackoff, jittered, capped at maxBackoff.
func retryBackoff(retry int, minBackoff, maxBackoff time.Duration) time.Duration {
	if minBackoff == 0 {
		return 0
	}
	if retry < 0 {
		return maxBackoff
	}
	// retry is the attempt index (1..MaxRetries), not a user-controlled width.
	backoff := minBackoff << uint(retry) //nolint:gosec // G115
	if backoff < minBackoff {
		return maxBackoff
	}
	span := int64(backoff)
	if span > 0 {
		// math/rand: tcp-session jitter MUST be stdlib math/rand (Yaegi; not crypto/rand, not rand/v2).
		backoff = minBackoff + time.Duration(rand.Int63n(span)) //nolint:gosec // G404
	}
	if backoff > maxBackoff || backoff < minBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// shouldRetry is go-redis shouldRetry as this client can see it. Timeouts and cancelled contexts are never retried. Pool wait elapsed is errPoolWait (same Error() text, not retried) so MaxRetries does not multiply PoolTimeout. Handshake AUTH or SELECT failures marked at dial are not retried.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if isCommandTimeout(err) {
		return false
	}
	if isHandshakeFailure(err) {
		return false
	}
	if isUnreachable(err) {
		return true
	}
	return isRetryableRedisReply(err)
}

// isHandshakeFailure is an AUTH or SELECT error marked at dial. Not retryable.
// Type assert, not errors.As: Yaegi panics on As for this struct (*target must implement error).
func isHandshakeFailure(err error) bool {
	_, ok := err.(handshakeFailure)
	return ok
}

// isCommandTimeout is redis:timeout from an I/O deadline or the overall command budget. Not retryable.
func isCommandTimeout(err error) bool {
	return err == errTimeout
}

// isUnreachable is redis:unreachable (EOF, unexpected EOF, dial failure, and other IO via ioError). Retryable unless the client is closed.
func isUnreachable(err error) bool {
	return err == errUnreachable
}

// isRetryableRedisReply is a Redis error reply go-redis retries: max clients, LOADING, READONLY, MASTERDOWN, CLUSTERDOWN, TRYAGAIN (space after the word).
func isRetryableRedisReply(err error) bool {
	text := err.Error()
	if text == "ERR max number of clients reached" {
		return true
	}
	for _, prefix := range []string{"LOADING ", "READONLY ", "MASTERDOWN ", "CLUSTERDOWN ", "TRYAGAIN "} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}
