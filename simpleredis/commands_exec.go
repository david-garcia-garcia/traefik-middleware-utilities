package simpleredis

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"
)

// exec borrows a connection, runs one RESP command, and retries retryable failures up to MaxRetries.
// INCR/INCRBY/EVAL can double-apply when a reply is lost and the command is sent again; that is accepted.
func (sr *SimpleRedis) exec(ctx context.Context, args ...[]byte) ([][]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maxRetries, minBackoff, maxBackoff := retryLimits(sr.maxRetries, sr.minRetryBackoff, sr.maxRetryBackoff)
	deadline := sr.commandDeadline(ctx, maxRetries)
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, errTimeout
		}
		if attempt > 0 {
			if err := waitUntil(ctx, deadline, retryBackoff(attempt, minBackoff, maxBackoff)); err != nil {
				return nil, err
			}
		}
		conn, err := sr.borrow(ctx, deadline)
		if err != nil {
			if sr.isClosed() || !shouldRetry(err) {
				return nil, err
			}
			last = err
			continue
		}
		values, reusable, err := sr.do(ctx, deadline, conn, args)
		if ctx.Err() != nil {
			sr.release(conn, false)
			return nil, ctx.Err()
		}
		sr.release(conn, reusable)
		if err == nil {
			return values, nil
		}
		if !shouldRetry(err) {
			return values, err
		}
		last = err
	}
	if last != nil {
		return nil, last
	}
	return nil, errTimeout
}

// commandDeadline is now plus (maxRetries+1)*(DialTimeout+IOTimeout), tightened by ctx if ctx has a sooner deadline.
func (sr *SimpleRedis) commandDeadline(ctx context.Context, maxRetries int) time.Time {
	budget := time.Duration(maxRetries+1) * (sr.DialTimeout() + sr.IOTimeout())
	deadline := time.Now().Add(budget)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

// waitUntil waits delay or until ctx/deadline fires. Deadline expiry is redis:timeout.
func waitUntil(ctx context.Context, deadline time.Time, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return errTimeout
	}
	if delay > remaining {
		delay = remaining
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		if time.Now().After(deadline) {
			return errTimeout
		}
		return nil
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

// shouldRetry is go-redis shouldRetry as this client can see it. Timeouts and cancelled contexts are never retried. Pool wait elapsed is errPoolWait (same Error() text, not retried) so MaxRetries does not multiply PoolTimeout.
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
	if isUnreachable(err) {
		return true
	}
	return isRetryableRedisReply(err)
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
