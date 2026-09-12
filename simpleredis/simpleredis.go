// Package simpleredis is a stdlib pooled TCP RESP client (GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL).
package simpleredis

import (
	"bufio"
	"crypto/sha1" //nolint:gosec // Redis EVALSHA digest is SHA-1
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Error strings for redis.
const (
	RedisUnreachable = "redis:unreachable"
	RedisMiss        = "redis:miss"
	RedisTimeout     = "redis:timeout"
	RedisNoAuth      = "redis:noauth"
	RedisIssue       = "redis:issue?"
)

const (
	maxIdleConns = 8
	idleTimeout  = 30 * time.Second
	dialTimeout  = 2 * time.Second
	ioTimeout    = 1 * time.Second

	// Redis EVAL / EVALSHA verbs and the NOSCRIPT miss prefix (after '-' is stripped).
	evalVerb       = "EVAL"
	evalShaVerb    = "EVALSHA"
	noScriptPrefix = "NOSCRIPT"
)

var (
	errUnreachable = errors.New(RedisUnreachable)
	errMiss        = errors.New(RedisMiss)
	errTimeout     = errors.New(RedisTimeout)
	errNoAuth      = errors.New(RedisNoAuth)
	errIssue       = errors.New(RedisIssue)
)

// pooledConn is one TCP socket plus RESP reader/writer kept in the idle list.
type pooledConn struct {
	netConn  net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	lastUsed time.Time
}

// close closes the TCP socket. Safe to call after a failed command.
func (c *pooledConn) close() {
	_ = c.netConn.Close()
}

// SimpleRedis is a pooled TCP RESP client. Init stores dial settings; commands dial on first use.
// MaxRetries, MinRetryBackoff, and MaxRetryBackoff follow go-redis Options sentinels so the zero value matches go-redis defaults: 0 means default (3 extra retries, 8ms, 512ms); -1 means off (no extra retries, no backoff sleep).
type SimpleRedis struct {
	host     string
	pass     string
	database string

	// MaxRetries is extra retries after the first attempt. 0 means 3; -1 means none (one send).
	MaxRetries int
	// MinRetryBackoff is the base backoff between retries. 0 means 8ms; -1 means no sleep.
	MinRetryBackoff time.Duration
	// MaxRetryBackoff caps backoff. 0 means 512ms; -1 means 0.
	MaxRetryBackoff time.Duration

	mu     sync.Mutex
	idle   []*pooledConn
	closed bool
}

// Close drains idle pooled connections and stops pooling. Further Get/Set/Del/MGet/Incr/IncrBy/Expire/ExpireAt/Eval return redis:unreachable and do not dial. In-flight commands still finish; their sockets are closed on release. Safe to call more than once.
func (sr *SimpleRedis) Close() {
	sr.mu.Lock()
	if sr.closed {
		sr.mu.Unlock()
		return
	}
	sr.closed = true
	idle := sr.idle
	sr.idle = nil
	sr.mu.Unlock()
	for _, conn := range idle {
		conn.close()
	}
}

// isClosed is true after Close. Used so a closed-client unreachable does not spin MaxRetries.
func (sr *SimpleRedis) isClosed() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.closed
}

// Init sets host, password, and database. Call once before concurrent use; not mutex-protected.
func (sr *SimpleRedis) Init(host, pass, database string) {
	sr.host = host
	sr.pass = pass
	sr.database = database
}

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
	maxRetries, minBackoff, maxBackoff := retryLimits(sr.MaxRetries, sr.MinRetryBackoff, sr.MaxRetryBackoff)
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff(attempt, minBackoff, maxBackoff))
		}
		conn, _, err := sr.borrow()
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

// shouldRetry is go-redis shouldRetry as this client can see it. Timeouts are never retried (documented deviation: ioTimeout is 1s). There is no pool wait, so there is no pool timeout to retry.
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

// borrow takes an idle socket younger than idleTimeout, or dials a new one.
func (sr *SimpleRedis) borrow() (*pooledConn, bool, error) {
	var reused *pooledConn
	var stale []*pooledConn
	now := time.Now()

	sr.mu.Lock()
	if sr.closed {
		sr.mu.Unlock()
		return nil, false, errUnreachable
	}
	// Idle sockets still inside idleTimeout are reused; older ones are closed after unlock.
	for len(sr.idle) > 0 {
		conn := sr.idle[len(sr.idle)-1]
		sr.idle = sr.idle[:len(sr.idle)-1]
		if now.Sub(conn.lastUsed) < idleTimeout {
			reused = conn
			break
		}
		stale = append(stale, conn)
	}
	sr.mu.Unlock()

	for _, conn := range stale {
		conn.close()
	}
	if reused != nil {
		return reused, true, nil
	}

	sr.mu.Lock()
	closed := sr.closed
	sr.mu.Unlock()
	if closed {
		return nil, false, errUnreachable
	}
	// Empty idle list: open a new TCP session (AUTH/SELECT in dial).
	conn, err := sr.dial()
	return conn, false, err
}

// release returns a clean conn to the idle list, or closes it when dirty, closed, or the idle cap is full.
func (sr *SimpleRedis) release(conn *pooledConn, reusable bool) {
	if !reusable {
		conn.close()
		return
	}
	conn.lastUsed = time.Now()

	sr.mu.Lock()
	if sr.closed || len(sr.idle) >= maxIdleConns {
		sr.mu.Unlock()
		conn.close()
		return
	}
	sr.idle = append(sr.idle, conn)
	sr.mu.Unlock()
}

// dial opens TCP to host, then AUTH and SELECT when those Init fields are set.
func (sr *SimpleRedis) dial() (*pooledConn, error) {
	dialer := net.Dialer{Timeout: dialTimeout}
	netConn, err := dialer.Dial("tcp", sr.host)
	if err != nil {
		return nil, errUnreachable
	}
	conn := &pooledConn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		writer:  bufio.NewWriter(netConn),
	}

	// AUTH before SELECT so a passworded server accepts the session.
	if sr.pass != "" {
		if _, _, err = sr.do(conn, [][]byte{[]byte("AUTH"), []byte(sr.pass)}); err != nil {
			conn.close()
			return nil, err
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(conn, [][]byte{[]byte("SELECT"), []byte(sr.database)}); err != nil {
			conn.close()
			return nil, err
		}
	}
	return conn, nil
}

// do writes one RESP command on conn and reads the reply. reusable is false when the socket is dirty.
func (sr *SimpleRedis) do(conn *pooledConn, args [][]byte) ([][]byte, bool, error) {
	if err := conn.netConn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
		return nil, false, errUnreachable
	}
	if err := writeCommand(conn.writer, args); err != nil {
		return nil, false, ioError(err)
	}
	values, clean, err := readReply(conn.reader)
	if err != nil && !clean {
		if err == errIssue {
			return nil, false, errIssue
		}
		return nil, false, ioError(err)
	}
	return values, true, err
}

// writeCommand writes one RESP array of bulk strings and flushes.
func writeCommand(writer *bufio.Writer, args [][]byte) error {
	if _, err := writer.WriteString("*" + strconv.Itoa(len(args)) + "\r\n"); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := writer.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n"); err != nil {
			return err
		}
		if _, err := writer.Write(arg); err != nil {
			return err
		}
		if _, err := writer.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// readReply parses one RESP value. clean is false when the stream is no longer usable.
func readReply(reader *bufio.Reader) ([][]byte, bool, error) {
	line, err := readLine(reader)
	if err != nil {
		return nil, false, err
	}
	if len(line) == 0 {
		return nil, false, errIssue
	}

	switch line[0] {
	case '+', ':':
		return [][]byte{line[1:]}, true, nil
	case '-':
		return nil, true, replyError(line[1:])
	case '$':
		data, bulkErr := readBulk(reader, line)
		if bulkErr == errMiss {
			return nil, true, errMiss
		}
		if bulkErr != nil {
			return nil, false, bulkErr
		}
		return [][]byte{data}, true, nil
	case '*':
		count, convErr := strconv.Atoi(string(line[1:]))
		if convErr != nil || count < 0 {
			return nil, false, errIssue
		}
		values := make([][]byte, count)
		for i := 0; i < count; i++ {
			head, headErr := readLine(reader)
			if headErr != nil {
				return nil, false, headErr
			}
			if len(head) == 0 {
				return nil, false, errIssue
			}
			switch head[0] {
			case '$':
				data, bulkErr := readBulk(reader, head)
				if bulkErr == errMiss {
					continue
				}
				if bulkErr != nil {
					return nil, false, bulkErr
				}
				values[i] = data
			case ':', '+':
				values[i] = head[1:]
			default:
				return nil, false, errIssue
			}
		}
		return values, true, nil
	default:
		return nil, false, errIssue
	}
}

// readBulk reads a $ payload (or a miss when length is negative).
func readBulk(reader *bufio.Reader, head []byte) ([]byte, error) {
	if len(head) == 0 || head[0] != '$' {
		return nil, errIssue
	}
	length, err := strconv.Atoi(string(head[1:]))
	if err != nil {
		return nil, errIssue
	}
	if length < 0 {
		return nil, errMiss
	}
	data := make([]byte, length+2)
	if _, err = io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data[:length], nil
}

// readLine reads one CRLF-terminated RESP line without the CRLF.
func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errIssue
	}
	return line[:len(line)-2], nil
}

// replyError maps AUTH-class Redis errors to redis:noauth and otherwise returns the payload text.
func replyError(message []byte) error {
	text := string(message)
	for _, prefix := range []string{"NOAUTH", "WRONGPASS", "NOPERM", "ERR Client sent AUTH"} {
		if strings.HasPrefix(text, prefix) {
			return errNoAuth
		}
	}
	return errors.New(text)
}

// ioError maps deadline exceeded to redis:timeout and other IO failures to redis:unreachable.
func ioError(err error) error {
	// errors.Is, not a net.Error assert: Yaegi has panicked on that interface across the interpreter boundary.
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return errTimeout
	}
	return errUnreachable
}
