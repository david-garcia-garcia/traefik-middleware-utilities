//go:build bugrepro

// Reproductions for simpleredis/BUGS.md (2026-09-13). Every test here encodes the spec and
// FAILS on current code. Run with:
//
//	go test -tags bugrepro -count=1 -timeout 120s -run TestBug ./simpleredis/
//
// The default suite does not build this file.
package simpleredis

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching proves that under Yaegi (the
// Traefik plugin runtime) a handshake AUTH/SELECT failure is an error no documented matcher can
// classify, because handshakeFailure's Unwrap is invisible to the compiled errors.Is.
//
// Spec (std_go_simpleredis_tcp-session, exported sentinels): callers match with errors.Is /
// IsUnreachable / IsMiss / IsPoolWait, not string equality. AUTH peer-close is redis:unreachable.
// Measured interpreted: Error()=="redis:unreachable" but IsUnreachable(err)==false.
func TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching(t *testing.T) {
	// Peer accepts TCP, reads AUTH, closes with no reply: dial wraps errUnreachable.
	_, addr := startAcceptFake(t, func(_ net.Conn, reader *bufio.Reader) {
		_, _ = readCommand(reader)
	})

	// Compiled control: the wrapper is transparent to errors.Is.
	compiled := New(Config{Host: addr, Pass: "secret", MaxRetries: -1})
	_, compiledErr := compiled.Get(context.Background(), "hit")
	if !IsUnreachable(compiledErr) {
		t.Fatalf("compiled control broke: IsUnreachable(%v) = false", compiledErr)
	}

	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "sentinelprobe", "sentinelprobe.go", bugSentinelProbeSrc)

	got := evalBugProbe(t, goPath, "sentinelprobe", fmt.Sprintf(`sentinelprobe.AuthEOFUnreachable(%q)`, addr))
	if got != "true" {
		t.Fatalf("interpreted IsUnreachable(handshake AUTH EOF) = %s, want true "+
			"(Error() is redis:unreachable, so every documented matcher must agree; "+
			"handshakeFailure.Unwrap is not visible to errors.Is under Yaegi)", got)
	}
}

// TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching is the same defect on the AUTH-class
// path: WRONGPASS surfaces as redis:noauth text that errors.Is cannot match when interpreted.
func TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("-WRONGPASS invalid password\r\n", statusOKReply)

	compiled := New(Config{Host: addr, Pass: "wrong", MaxRetries: -1})
	_, compiledErr := compiled.Get(context.Background(), "hit")
	if !bugErrorsIsNoAuth(compiledErr) {
		t.Fatalf("compiled control broke: errors.Is(%v, ErrNoAuth) = false", compiledErr)
	}

	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathFile(t, goPath, "sentinelprobe", "sentinelprobe.go", bugSentinelProbeSrc)

	got := evalBugProbe(t, goPath, "sentinelprobe", fmt.Sprintf(`sentinelprobe.AuthRejectNoAuth(%q)`, addr))
	if got != "true" {
		t.Fatalf("interpreted errors.Is(handshake WRONGPASS, ErrNoAuth) = %s, want true", got)
	}
}

// TestBugLostInUseTurnBricksPoolPermanently proves one lost in-use turn is never recovered, so
// PoolSize recovered panics between borrow and release brick the client for the process lifetime.
//
// Spec (std_go_simpleredis_tcp-session, Idle connections are pooled): live sockets (idle plus
// checked out) SHALL not exceed PoolSize, and a caller waits for a released socket instead of
// dialing. With zero sockets actually live, refusing to dial is not backpressure, it is a dead
// client. exec releases without defer on purpose, so a panic inside do never returns the turn.
// Measured: after PoolSize recovered panics the in-use-turn channel is 0/PoolSize and every later
// command returns redis:unreachable forever.
func TestBugLostInUseTurnBricksPoolPermanently(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	const poolSize = 2
	sr := New(Config{
		Host:        addr,
		PoolSize:    poolSize,
		PoolTimeout: 20 * time.Millisecond,
		MaxRetries:  -1,
	})
	if _, err := sr.Get(context.Background(), "hit"); err != nil {
		t.Fatalf("warm Get: %v", err)
	}

	// Exactly what a panic inside do costs under Traefik: the request is recovered and the
	// non-deferred release never runs.
	for i := 0; i < poolSize; i++ {
		borrowErr := bugPanicAfterBorrow(sr)
		if borrowErr != nil {
			t.Fatalf("borrow %d: %v", i, borrowErr)
		}
	}

	// No socket the client owns is live any more, so a dial is within the frozen cap.
	_, err := sr.Get(context.Background(), "hit")
	if err != nil {
		t.Fatalf("Get after %d lost in-use turns = %v, want success: turns=%d/%d, idle=%d, accepts=%d "+
			"(a leaked turn is permanent; there is no reaper and OverFrees does not recover it)",
			poolSize, err, len(sr.inUseTurns), cap(sr.inUseTurns), pooledIdle(sr), fake.connections())
	}
}

// TestBugDesyncedSocketKeepsServingPreviousReplies proves a pooled socket that carries one
// unread reply is never evicted, so every later command on it returns the previous command's
// value. release refreshes lastUsed on each use, so IdleTimeout never fires under traffic.
//
// Spec (std_go_simpleredis_resp-commands, Get): Get returns the value for its own key. A socket
// the decoder cannot prove is on a reply boundary MUST NOT keep serving commands.
// Measured: Get(k6) returned v5, and the poisoning never healed across 40 commands.
func TestBugDesyncedSocketKeepsServingPreviousReplies(t *testing.T) {
	// Compliant framing, but the 5th command draws one extra unsolicited bulk reply, the shape a
	// RESP3 push or a duplicating proxy produces.
	addr := startStrayExtraReplyFake(t, 5)
	sr := New(Config{Host: addr, PoolSize: 1, MaxRetries: -1})

	const commands = 12
	for i := 0; i < commands; i++ {
		key := fmt.Sprintf("k%d", i)
		want := "v" + key[1:]
		got, err := sr.Get(context.Background(), key)
		if err != nil {
			// An error is an acceptable outcome: it discards the socket instead of lying.
			continue
		}
		if string(got) != want {
			t.Fatalf("Get(%s) = %q, want %q or an error (command %d): a desynced socket stayed in "+
				"the pool and every later command reads the previous command's reply",
				key, got, want, i)
		}
	}
}

// bugPanicAfterBorrow takes an in-use turn and panics, recovering like Traefik does. The turn is
// never returned. It reports a borrow failure, if any.
func bugPanicAfterBorrow(sr *SimpleRedis) (borrowErr error) {
	defer func() { _ = recover() }()
	conn, err := sr.borrow(context.Background())
	if err != nil {
		return err
	}
	_ = conn
	panic("simulated panic inside do")
}

// bugErrorsIsNoAuth is errors.Is(err, ErrNoAuth) without importing errors into this file twice.
func bugErrorsIsNoAuth(err error) bool {
	for err != nil {
		if err == ErrNoAuth {
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// evalBugProbe imports pkg into a fresh Yaegi interpreter (stdlib only, no unsafe) and evaluates expr.
func evalBugProbe(t *testing.T, goPath, pkg, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "` + pkg + `"`); err != nil {
		t.Fatalf("import %s: %v", pkg, err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// startStrayExtraReplyFake answers GET kN with vN and, every nth command, appends one extra
// unsolicited bulk reply so the socket is left one reply ahead.
func startStrayExtraReplyFake(t *testing.T, nth int) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				seen := 0
				for {
					args, readErr := readCommand(reader)
					if readErr != nil {
						return
					}
					seen++
					value := "v" + args[1][1:]
					_, _ = io.WriteString(conn, fmt.Sprintf("$%d\r\n%s\r\n", len(value), value))
					if seen%nth == 0 {
						_, _ = io.WriteString(conn, "$5\r\nSTRAY\r\n")
					}
				}
			}(conn)
		}
	}()
	return listener.Addr().String()
}

// bugSentinelProbeSrc is the interpreted probe: it reports how plugin code sees a handshake error.
const bugSentinelProbeSrc = `package sentinelprobe

import (
	"context"
	"errors"
	"strconv"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// AuthEOFUnreachable reports IsUnreachable for an AUTH peer-close handshake failure.
func AuthEOFUnreachable(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, Pass: "secret", MaxRetries: -1})
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if err.Error() != simpleredis.RedisUnreachable {
		return "text:" + err.Error()
	}
	return strconv.FormatBool(simpleredis.IsUnreachable(err))
}

// AuthRejectNoAuth reports errors.Is(err, ErrNoAuth) for a WRONGPASS handshake failure.
func AuthRejectNoAuth(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, Pass: "wrong", MaxRetries: -1})
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if err.Error() != simpleredis.RedisNoAuth {
		return "text:" + err.Error()
	}
	return strconv.FormatBool(errors.Is(err, simpleredis.ErrNoAuth))
}
`
