package simpleredis

import (
	"crypto/sha1" //nolint:gosec // Redis EVALSHA digest is SHA-1
	"encoding/hex"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	// Redis EVAL / EVALSHA verbs and the NOSCRIPT miss prefix (after '-' is stripped).
	evalVerb       = "EVAL"
	evalShaVerb    = "EVALSHA"
	noScriptPrefix = "NOSCRIPT"
)

// Get fetches the value for key name in redis.
func (sr *SimpleRedis) Get(name string) ([]byte, error) {
	values, err := sr.exec([]byte("GET"), []byte(name))
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, errIssue
	}
	return values[0], nil
}

// MGet fetches the values for keys names in redis, nil where a key is missing.
func (sr *SimpleRedis) MGet(names []string) ([][]byte, error) {
	if len(names) == 0 {
		return nil, nil
	}
	args := make([][]byte, 0, len(names)+1)
	args = append(args, []byte("MGET"))
	for _, name := range names {
		args = append(args, []byte(name))
	}
	values, err := sr.exec(args...)
	if err != nil {
		return nil, err
	}
	if len(values) != len(names) {
		return nil, errIssue
	}
	return values, nil
}

// Set updates the value for key name in redis with value data for duration.
func (sr *SimpleRedis) Set(name string, data []byte, duration int64) error {
	_, err := sr.exec([]byte("SET"), []byte(name), data, []byte("EX"), []byte(strconv.FormatInt(duration, 10)))
	return err
}

// Del removes the key name in redis.
func (sr *SimpleRedis) Del(name string) error {
	_, err := sr.exec([]byte("DEL"), []byte(name))
	return err
}

// Incr adds one to key name and returns the integer after the increment.
func (sr *SimpleRedis) Incr(name string) (int64, error) {
	return parseIntegerReply(sr.exec([]byte("INCR"), []byte(name)))
}

// IncrBy adds delta to key name and returns the integer after the increment.
func (sr *SimpleRedis) IncrBy(name string, delta int64) (int64, error) {
	return parseIntegerReply(sr.exec([]byte("INCRBY"), []byte(name), []byte(strconv.FormatInt(delta, 10))))
}

// Expire sets a TTL in seconds on key name. Integer 0 or 1 is success.
func (sr *SimpleRedis) Expire(name string, seconds int64) error {
	_, err := sr.exec([]byte("EXPIRE"), []byte(name), []byte(strconv.FormatInt(seconds, 10)))
	return err
}

// ExpireAt sets an absolute Unix expiry on key name. Integer 0 or 1 is success.
func (sr *SimpleRedis) ExpireAt(name string, unixSeconds int64) error {
	_, err := sr.exec([]byte("EXPIREAT"), []byte(name), []byte(strconv.FormatInt(unixSeconds, 10)))
	return err
}

// Eval runs a Lua script with KEYS then ARGV. Hashes the body each call and sends EVALSHA; on NOSCRIPT falls back once to EVAL.
func (sr *SimpleRedis) Eval(script string, keys []string, args []string) ([][]byte, error) {
	// Hash every call: SHA-1 of a limiter script (~470 B) is cheaper than a mutex, and a map of bodies would need a lock because Go maps are not concurrent.
	digest := scriptSHA1Hex(script)
	values, err := sr.exec(evalArgv(evalShaVerb, digest, keys, args)...)
	// Miss: engine has no matching digest (FLUSH, restart); EVAL is the only send of the body so the engine stores it.
	if err != nil && strings.HasPrefix(err.Error(), noScriptPrefix) {
		return sr.exec(evalArgv(evalVerb, script, keys, args)...)
	}
	return values, err
}

const maxMSetEXPairs = 1024

// groupWritePath is whether this client sends native MSETEX or the Lua fallback.
type groupWritePath int

const (
	groupWriteUnknown groupWritePath = iota
	groupWriteNative
	groupWriteLua
)

// msetexFallbackScript sets each KEYS[i] to ARGV[i] with EX or EXAT from the last two ARGV.
// Lua 5.1-safe: numeric for, no unpack / table.unpack / table.maxn. Keys stay in KEYS for Dragonfly.
const msetexFallbackScript = `local token = ARGV[#ARGV - 1]
local ttl = ARGV[#ARGV]
for i = 1, #KEYS do
  redis.call('SET', KEYS[i], ARGV[i], token, ttl)
end
return 1`

// MSetEX writes names and values with one shared TTL in seconds (native MSETEX or Lua fallback).
func (sr *SimpleRedis) MSetEX(names []string, values [][]byte, seconds int64) error {
	return sr.msetex(names, values, "EX", seconds)
}

// MSetEXAt writes names and values with one shared Unix expiry (native MSETEX or Lua fallback).
func (sr *SimpleRedis) MSetEXAt(names []string, values [][]byte, unixSeconds int64) error {
	return sr.msetex(names, values, "EXAT", unixSeconds)
}

// msetex validates the pair lists then sends native MSETEX, falling back to Eval on unknown-command.
func (sr *SimpleRedis) msetex(names []string, values [][]byte, expireToken string, ttl int64) error {
	if len(names) == 0 || len(names) != len(values) || len(names) > maxMSetEXPairs {
		return errIssue
	}
	if sr.cachedGroupWrite() == groupWriteLua {
		return sr.msetexEval(names, values, expireToken, ttl)
	}
	n, err := parseIntegerReply(sr.exec(msetexArgs(names, values, expireToken, ttl)...))
	if unknownCommand(err) {
		sr.storeGroupWrite(groupWriteLua)
		return sr.msetexEval(names, values, expireToken, ttl)
	}
	if err == nil {
		sr.storeGroupWrite(groupWriteNative)
	}
	return msetexSuccess(n, err)
}

// msetexEval runs the fallback script with names in KEYS and values then token then TTL in ARGV.
func (sr *SimpleRedis) msetexEval(names []string, values [][]byte, expireToken string, ttl int64) error {
	argv := make([]string, 0, len(values)+2)
	for _, value := range values {
		argv = append(argv, string(value))
	}
	argv = append(argv, expireToken, strconv.FormatInt(ttl, 10))
	return msetexSuccess(parseIntegerReply(sr.Eval(msetexFallbackScript, names, argv)))
}

// cachedGroupWrite returns the capability cache. Callers must not hold groupWriteMu.
func (sr *SimpleRedis) cachedGroupWrite() groupWritePath {
	sr.groupWriteMu.Lock()
	path := sr.groupWrite
	sr.groupWriteMu.Unlock()
	return path
}

// storeGroupWrite records native or lua for this client. Callers must not hold groupWriteMu.
func (sr *SimpleRedis) storeGroupWrite(path groupWritePath) {
	sr.groupWriteMu.Lock()
	sr.groupWrite = path
	sr.groupWriteMu.Unlock()
}

// msetexArgs is native MSETEX: numkeys, pairs in order, then EX or EXAT, then the decimal TTL.
func msetexArgs(names []string, values [][]byte, expireToken string, ttl int64) [][]byte {
	args := make([][]byte, 0, 4+2*len(names))
	args = append(args, []byte("MSETEX"), []byte(strconv.Itoa(len(names))))
	for i, name := range names {
		args = append(args, []byte(name), values[i])
	}
	args = append(args, []byte(expireToken), []byte(strconv.FormatInt(ttl, 10)))
	return args
}

// msetexSuccess is nil when the engine returned integer 1. Other integers are redis:issue?.
func msetexSuccess(n int64, err error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return errIssue
	}
	return nil
}

// unknownCommand is true when Redis or Dragonfly rejected the verb as not in the command table.
func unknownCommand(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "ERR unknown command")
}

// scriptSHA1Hex is Redis sha1hex of the script bytes (lowercase 40-char hex). Eval hashes each call; no client digest table.
func scriptSHA1Hex(script string) string {
	sum := sha1.Sum([]byte(script)) //nolint:gosec // Redis EVALSHA digest is SHA-1
	return hex.EncodeToString(sum[:])
}

// evalArgv builds EVAL or EVALSHA argv: verb, script-or-digest, decimal numkeys, keys, then args.
func evalArgv(verb, scriptOrDigest string, keys, args []string) [][]byte {
	wire := make([][]byte, 0, 3+len(keys)+len(args))
	wire = append(wire, []byte(verb), []byte(scriptOrDigest), []byte(strconv.Itoa(len(keys))))
	for _, key := range keys {
		wire = append(wire, []byte(key))
	}
	for _, arg := range args {
		wire = append(wire, []byte(arg))
	}
	return wire
}

// parseIntegerReply reads one decimal integer from a : reply. Garbage payload is redis:issue?.
func parseIntegerReply(values [][]byte, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if len(values) != 1 {
		return 0, errIssue
	}
	n, convErr := strconv.ParseInt(string(values[0]), 10, 64)
	if convErr != nil {
		return 0, errIssue
	}
	return n, nil
}

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
