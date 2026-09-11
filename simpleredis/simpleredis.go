// Package simpleredis is a stdlib pooled TCP RESP client (GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL).
package simpleredis

import (
	"bufio"
	"errors"
	"io"
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
type SimpleRedis struct {
	host     string
	pass     string
	database string

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

// Eval runs a Lua script with KEYS then ARGV. numkeys is len(keys).
func (sr *SimpleRedis) Eval(script string, keys []string, args []string) ([][]byte, error) {
	wire := make([][]byte, 0, 3+len(keys)+len(args))
	wire = append(wire, []byte("EVAL"), []byte(script), []byte(strconv.Itoa(len(keys))))
	for _, key := range keys {
		wire = append(wire, []byte(key))
	}
	for _, arg := range args {
		wire = append(wire, []byte(arg))
	}
	return sr.exec(wire...)
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

// exec borrows a connection, runs one RESP command, and retries once when a reused idle socket is dead.
func (sr *SimpleRedis) exec(args ...[]byte) ([][]byte, error) {
	conn, reused, err := sr.borrow()
	if err != nil {
		return nil, err
	}
	values, reusable, err := sr.do(conn, args)
	sr.release(conn, reusable)
	// Timeouts are not retried: a stalled peer will stall the next dial too.
	if err == nil || reusable || !reused || err == errTimeout {
		return values, err
	}
	// Dead pooled conn: borrow again so Close cannot skip the closed check.
	conn, _, err = sr.borrow()
	if err != nil {
		return nil, err
	}
	values, reusable, err = sr.do(conn, args)
	sr.release(conn, reusable)
	return values, err
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

// release peels idle-head sockets older than idleTimeout, then returns a clean conn to the idle list, or closes it when dirty, closed, or the idle cap is full.
func (sr *SimpleRedis) release(conn *pooledConn, reusable bool) {
	if !reusable {
		conn.close()
		return
	}
	conn.lastUsed = time.Now()
	now := conn.lastUsed

	sr.mu.Lock()
	// Close idle-head sockets older than idleTimeout; stop at the first still-valid head.
	var peeled []*pooledConn
	for len(sr.idle) > 0 {
		head := sr.idle[0]
		if now.Sub(head.lastUsed) < idleTimeout {
			break
		}
		peeled = append(peeled, head)
		sr.idle = sr.idle[1:]
	}
	poolClosed := sr.closed
	idleFull := len(sr.idle) >= maxIdleConns
	if !poolClosed && !idleFull {
		sr.idle = append(sr.idle, conn)
	}
	sr.mu.Unlock()

	for _, peeledConn := range peeled {
		peeledConn.close()
	}
	if poolClosed || idleFull {
		conn.close()
	}
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
