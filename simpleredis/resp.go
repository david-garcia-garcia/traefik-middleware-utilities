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
		// Copy: ReadSlice view is invalid after the next read or idle release.
		return [][]byte{append([]byte(nil), line[1:]...)}, true, nil
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
		count, ok := parseLen(line[1:])
		if !ok || count < 0 {
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
				values[i] = append([]byte(nil), head[1:]...)
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
	length, ok := parseLen(head[1:])
	if !ok {
		return nil, errIssue
	}
	if length < 0 {
		return nil, errMiss
	}
	data := make([]byte, length+2)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data[:length], nil
}

// readLine reads one CRLF-terminated RESP line without the CRLF.
func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		if err != bufio.ErrBufferFull {
			return nil, err
		}
		// Partial aliases the bufio buffer; copy before the remainder read.
		full := make([]byte, len(line))
		copy(full, line)
		remainder, remainderErr := reader.ReadBytes('\n')
		if remainderErr != nil {
			return nil, remainderErr
		}
		line = append(full, remainder...)
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errIssue
	}
	return line[:len(line)-2], nil
}

const maxParseLen = int(^uint(0) >> 1)

// parseLen parses a RESP length from the bytes after the type byte.
// An optional leading minus is accepted so $-1 stays a miss. Empty or non-digit input is false.
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
