package simpleredis

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// do writes one RESP command on conn and reads the reply. reusable is false when the socket is dirty,
// leftover bytes remain after a complete value, or unread bytes were already in the reader before the write.
// exec calls do from runOnConn, which defers release so a panic still returns the in-use turn and closes the socket.
func (sr *SimpleRedis) do(ctx context.Context, conn *pooledConn, args [][]byte) (reply [][]byte, reusable bool, err error) {
	if err := contextStop(ctx); err != nil {
		return nil, false, err
	}
	ioBound := clampTimeout(ctx, sr.IOTimeout())
	if ioBound <= 0 {
		if err := contextStop(ctx); err != nil {
			return nil, false, err
		}
		return nil, false, errTimeout
	}
	if err := conn.netConn.SetDeadline(time.Now().Add(ioBound)); err != nil {
		return nil, false, errUnreachable
	}
	// net.Conn Read/Write ignore ctx; Close on cancel is what unblocks them before SetDeadline.
	stopWatch := watchConnClose(ctx, conn.netConn)
	defer stopWatch()
	// Unread leftover from a prior command on this socket must not be parsed as this command's reply.
	// errUnreachable, not errIssue: the socket is unusable but the command is not, so this must stay
	// retryable — a retry gets a fresh dial whose reader is empty by construction. Same call the spec
	// makes for a short bulk read, which MUST NOT surface as redis:issue?.
	// Buffered() covers leftover already in the reader, not a stray that arrives while the socket is idle: it does not see the kernel receive buffer.
	if conn.reader.Buffered() != 0 {
		return nil, false, errUnreachable
	}
	if err := writeCommand(conn.writer, args); err != nil {
		return nil, false, ioOrContext(ctx, ioBound, sr.IOTimeout(), err)
	}
	values, clean, err := readReply(conn.reader)
	if stop := contextStop(ctx); stop != nil {
		return nil, false, stop
	}
	if err != nil && !clean {
		if isDirtyProtocolError(err) {
			return nil, false, err
		}
		return nil, false, ioOrContext(ctx, ioBound, sr.IOTimeout(), err)
	}
	// Extra well-formed RESP after this reply means the peer is ahead; return the value and destroy the socket.
	// Same limit as the pre-write check: leftover already in the reader, not a stray still only in the kernel buffer.
	if conn.reader.Buffered() != 0 {
		return values, false, err
	}
	return values, true, err
}

// watchConnClose closes conn when ctx is done so a blocked read returns. Stop the returned func when I/O finishes.
//
// Why AfterFunc, not go + select on ctx.Done: this package is interpreted under Yaegi (Traefik
// plugins and TestYaegi_*). Yaegi v0.16.1's interp._select races when interpreted code selects on
// a context channel from a goroutine. CI `go test -race` failed that way on TestYaegi_NewGetSetDel,
// TestYaegi_AllowBurst, and TestYaegi_TakeUntilDeny once exec started wrapping Background with
// WithDeadline (Background.Done is nil, so the old select never armed; a deadline ctx does).
// context.AfterFunc is the real stdlib waiter (Yaegi maps the symbol to context.AfterFunc), so the
// race detector does not see Yaegi's select.
func watchConnClose(ctx context.Context, conn net.Conn) func() {
	// Background and other never-done ctx: nothing to wait for; AfterFunc would park a goroutine until Stop.
	if ctx.Done() == nil {
		return func() {}
	}
	stop := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	return func() { stop() }
}

// ioOrContext prefers a fired context over a socket I/O error.
//
// Why not ioError alone: SetDeadline is a wall clock. When ioBound was clamped to ctx's remaining
// time, the OS can return os.ErrDeadlineExceeded a few ms before ctx.Done closes. ioError maps
// that to redis:timeout, so a caller deadline looked like the library budget. Go E2E Redis failed
// TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout that way. If the socket timeout is the
// clamped ctx instant, return DeadlineExceeded and let exec map library vs caller.
func ioOrContext(ctx context.Context, ioBound, ioTimeout time.Duration, err error) error {
	if stop := contextStop(ctx); stop != nil {
		return stop
	}
	if ioBound < ioTimeout && errors.Is(err, os.ErrDeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return ioError(err)
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
func readReply(reader *bufio.Reader) (values [][]byte, clean bool, err error) {
	line, err := readLine(reader)
	if err != nil {
		return nil, false, err
	}
	if len(line) == 0 {
		return nil, false, errIssue
	}

	switch line[0] {
	case '+', ':':
		// Copy: ReadSlice view is invalid after the next read or idle release.
		return [][]byte{append([]byte(nil), line[1:]...)}, true, nil
	case '-':
		return nil, true, replyError(line[1:])
	case '$':
		data, bulkErr := readBulk(reader, line)
		if bulkErr == errMiss { //nolint:errorlint // readBulk returns errMiss as the exact $-1 sentinel; a wrap is not that decode miss
			return [][]byte{nil}, true, nil
		}
		if bulkErr != nil {
			return nil, false, bulkErr
		}
		return [][]byte{data}, true, nil
	case '*':
		count, ok := parseLen(line[1:])
		// *-1 is a legal RESP2 nil array (BLPOP timeout, EXEC abort); this client has no verb that receives it, so it is redis:issue? not redis:miss.
		if !ok || count < 0 {
			return nil, false, errIssue
		}
		// Over-cap * must not make; a later short read retries as unreachable.
		if count > maxArrayCount {
			return nil, false, errIssue
		}
		// Do not make(count): a *1048576 header with no elements would allocate ~24 MiB of slot headers.
		startCap := count
		if startCap > arrayGrowChunk {
			startCap = arrayGrowChunk
		}
		values := make([][]byte, 0, startCap)
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
				if bulkErr == errMiss { //nolint:errorlint // readBulk returns errMiss as the exact $-1 sentinel; a wrap is not that decode miss
					values = append(values, nil)
					continue
				}
				if bulkErr != nil {
					return nil, false, bulkErr
				}
				values = append(values, data)
			case ':', '+':
				values = append(values, append([]byte(nil), head[1:]...))
			default:
				// Nested array, error-in-array, or other element type this decoder does not decode.
				return nil, false, errUnsupportedReply
			}
		}
		return values, true, nil
	default:
		// Unknown type byte (HTTP-shaped, RESP3, garbage). Well-framed enough to refuse, not to parse.
		return nil, false, errUnsupportedReply
	}
}

// isDirtyProtocolError is a framing or unsupported-type sentinel. It must not become redis:unreachable.
func isDirtyProtocolError(err error) bool {
	return err == errIssue || err == errUnsupportedReply //nolint:errorlint // dirty-protocol sentinels this decoder returns exactly; a wrap would be a different I/O failure
}

// readBulk reads a $ payload (or a miss when length is negative) and requires a CRLF trailer.
func readBulk(reader *bufio.Reader, head []byte) ([]byte, error) {
	if len(head) == 0 || head[0] != '$' {
		return nil, errIssue
	}
	length, ok := parseLen(head[1:])
	if !ok {
		return nil, errIssue
	}
	if length < 0 {
		return nil, errMiss
	}
	// Over-cap headers must not allocate; truncated ReadFull would retry as unreachable.
	if length > maxBulkLength {
		return nil, errIssue
	}
	// Do not make(length+2): an in-cap header with no payload would allocate tens of MiB from a handful of bytes.
	// Read a first chunk instead, so a lying header costs at most bulkReadChunk.
	need := length + 2
	first := need
	if first > bulkReadChunk {
		first = bulkReadChunk
	}
	data := make([]byte, first)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	if need > first {
		// The peer delivered the first chunk, so size the remainder in one step. Growing by
		// repeated append instead would recopy the payload and cost several times its size.
		full := make([]byte, need)
		copy(full, data)
		data = full
		if _, err := io.ReadFull(reader, data[first:]); err != nil {
			return nil, err
		}
	}
	// Trailer must be CRLF; otherwise the stream is off a reply boundary.
	if data[length] != '\r' || data[length+1] != '\n' {
		return nil, errIssue
	}
	return data[:length], nil
}

// readLine reads one CRLF-terminated RESP line without the CRLF.
// A line that fills the bufio buffer without a newline is redis:issue? and is not grown.
func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		if err == bufio.ErrBufferFull { //nolint:errorlint // ReadSlice returns ErrBufferFull exactly; a wrap would be I/O and must stay unreachable
			return nil, errIssue
		}
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errIssue
	}
	return line[:len(line)-2], nil
}

const (
	maxBulkLength  = 64 << 20 // largest $ payload this decoder will allocate
	maxArrayCount  = 1 << 20  // largest * count this decoder will allocate
	maxParseLen    = int(^uint(0) >> 1)
	bulkReadChunk  = 128 << 10 // first make is min(need, this); a 64 MiB header must not make 64 MiB
	arrayGrowChunk = 16        // * start cap; a 1 Mi header must not make 1 Mi slot headers
)

// parseLen parses a RESP length from the bytes after the type byte.
// An optional leading minus is accepted so $-1 and *-1 parse as negative. Empty or non-digit input is false.
func parseLen(digits []byte) (int, bool) {
	if len(digits) == 0 {
		return 0, false
	}
	i := 0
	negative := false
	if digits[0] == '-' {
		negative = true
		i++
		if i == len(digits) {
			return 0, false
		}
	}
	length := 0
	for ; i < len(digits); i++ {
		digitByte := digits[i]
		if digitByte < '0' || digitByte > '9' {
			return 0, false
		}
		digit := int(digitByte - '0')
		if length > (maxParseLen-digit)/10 {
			return 0, false
		}
		length = length*10 + digit
	}
	if negative {
		return -length, true
	}
	return length, true
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
