package simpleredis

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// do writes one RESP command on conn and reads the reply. reusable is false when the socket is dirty.
func (sr *SimpleRedis) do(conn *pooledConn, args [][]byte) ([][]byte, bool, error) {
	if err := conn.netConn.SetDeadline(time.Now().Add(sr.ioTimeout)); err != nil {
		return nil, false, errUnreachable
	}
	if err := writeCommand(conn, args); err != nil {
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

// appendRESP appends one RESP array of bulk strings onto buf using dest framing.
func appendRESP(buf []byte, args [][]byte) []byte {
	// Array header: *<count>\r\n
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)), 10)
	buf = append(buf, '\r', '\n')
	// Each arg is a bulk string: $<len>\r\n<payload>\r\n
	for _, arg := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(arg)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, arg...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

// writeCommand encodes args into conn.buf and issues one net.Conn.Write.
func writeCommand(conn *pooledConn, args [][]byte) error {
	buf := appendRESP(conn.buf[:0], args)
	conn.buf = buf
	// One Write; a short write or error leaves the socket dirty.
	n, err := conn.netConn.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return io.ErrShortWrite
	}
	return nil
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
