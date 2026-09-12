package simpleredis

import (
	"math/rand"
	"strings"
	"time"
)

// exec borrows a connection, runs one RESP command, and retries retryable failures up to MaxRetries.
// INCR/INCRBY/EVAL can double-apply when a reply is lost and the command is sent again; that is accepted.
func (sr *SimpleRedis) exec(args ...[]byte) ([][]byte, error) {
	maxRetries, minBackoff, maxBackoff := retryLimits(sr.maxRetries, sr.minRetryBackoff, sr.maxRetryBackoff)
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff(attempt, minBackoff, maxBackoff))
		}
		conn, err := sr.borrow()
		if err != nil {
			if sr.isClosed() || !shouldRetry(err) {
				return nil, err
			}
			last = err
			continue
		}
		// If do panics under Yaegi, the process does not crash and this in-use-turn is lost.
		// Not deferred-release: https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/29
		values, reusable, err := sr.do(conn, args)
		sr.release(conn, reusable)
		if err == nil {
			return values, nil
		}
		if !shouldRetry(err) {
			return values, err
		}
		last = err
	}
	return nil, last
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

// shouldRetry is go-redis shouldRetry as this client can see it. Timeouts are never retried (documented deviation: IOTimeout default is 1s). Pool wait elapsed is errPoolWait (same Error() text, not retried) so MaxRetries does not multiply PoolTimeout.
func shouldRetry(err error) bool {
	if err == nil {
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

// isCommandTimeout is redis:timeout from an I/O deadline. Not retryable.
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
