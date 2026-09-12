// Command respdroprelay forwards RESP to an upstream engine and drops INCR/INCRBY/EVAL replies after the session has already forwarded at least one command.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// main listens on :6379 and relays each accepted client to UPSTREAM.
func main() {
	upstream := os.Getenv("UPSTREAM")
	if upstream == "" {
		log.Fatal("UPSTREAM is required")
	}
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
	}
	for {
		client, err := listener.Accept()
		if err != nil {
			return
		}
		go relaySession(client, upstream)
	}
}

// relaySession dials upstream, then forwards one RESP command at a time until a dropped verb or IO error.
func relaySession(client net.Conn, upstreamAddr string) {
	defer client.Close()
	upstream, err := net.Dial("tcp", upstreamAddr)
	if err != nil {
		return
	}
	defer upstream.Close()

	clientReader := bufio.NewReader(client)
	upstreamReader := bufio.NewReader(upstream)
	upstreamWriter := bufio.NewWriter(upstream)
	hasForwarded := false
	for {
		args, err := readCommand(clientReader)
		if err != nil {
			return
		}
		if err := writeCommand(upstreamWriter, args); err != nil {
			return
		}
		reply, err := readRawRESP(upstreamReader)
		if err != nil {
			return
		}
		// Drop only after this TCP session already forwarded a command (probe warms with GET). A retry on a new session whose first command is INCR/EVAL must pass the reply through.
		if hasForwarded && verbDropsReply(args[0]) {
			return
		}
		if _, err := client.Write(reply); err != nil {
			return
		}
		hasForwarded = true
	}
}

// verbDropsReply is true for INCR, INCRBY, and EVAL so the engine apply is kept and the client sees a dead socket.
func verbDropsReply(verb string) bool {
	switch strings.ToUpper(verb) {
	case "INCR", "INCRBY", "EVAL":
		return true
	default:
		return false
	}
}

// readCommand parses one RESP array of bulk strings from the client.
func readCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(header) < 3 || header[0] != '*' {
		return nil, io.ErrUnexpectedEOF
	}
	count, err := strconv.Atoi(header[1 : len(header)-2])
	if err != nil {
		return nil, err
	}
	args := make([]string, count)
	for i := 0; i < count; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line[1 : len(line)-2])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, length+2)
		if _, err = io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:length])
	}
	return args, nil
}

// writeCommand writes one RESP array of bulk strings and flushes.
func writeCommand(writer *bufio.Writer, args []string) error {
	if _, err := writer.WriteString("*" + strconv.Itoa(len(args)) + "\r\n"); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := writer.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n"); err != nil {
			return err
		}
		if _, err := writer.WriteString(arg); err != nil {
			return err
		}
		if _, err := writer.WriteString("\r\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// readRawRESP copies one complete RESP value from reader, including nested arrays.
func readRawRESP(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, io.ErrUnexpectedEOF
	}
	switch line[0] {
	case '+', '-', ':':
		return line, nil
	case '$':
		n, err := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return line, nil
		}
		payload := make([]byte, n+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(line)+len(payload))
		out = append(out, line...)
		return append(out, payload...), nil
	case '*':
		n, err := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return line, nil
		}
		raw := append([]byte{}, line...)
		for i := 0; i < n; i++ {
			part, err := readRawRESP(reader)
			if err != nil {
				return nil, err
			}
			raw = append(raw, part...)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("bad RESP type %q", line[0])
	}
}
