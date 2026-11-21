package simpleredis

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"
)

// Handshake verbs dial sends on a new socket before the caller's command.
// do also names them: unread bytes before one of these is handshake leftover, not a stray reply.
const (
	verbAuth   = "AUTH"
	verbSelect = "SELECT"
)

// pooledConn is one TCP socket plus RESP reader/writer kept in idleConns.
//
// The writer stays a bufio.Writer. Encoding the whole command into a growable per-conn scratch
// and issuing one net.Conn.Write was implemented and measured, and rejected: bufio already
// coalesces a small command (the workload here is counters, TTLs and short Lua) into exactly one
// write, so the scratch bought no syscall, while it copied every payload an extra time and needed
// a 64 KiB idle-retention cap that had to be specified and tested. Compiled 100 KiB SET encode:
// 1590 ns with the scratch, 65 ns with bufio, because bufio hands a large slice straight to the
// socket instead of copying it. See writeCommand in resp.go for where the allocation win is.
type pooledConn struct {
	netConn net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	// lenBuf holds the decimal digits of one RESP length header; 24 bytes takes any int64.
	// It is owned by the connection on purpose; writeCommand says why a local array is not.
	lenBuf   [24]byte
	lastUsed time.Time
}

// close closes the TCP socket. Safe to call after a failed command.
func (c *pooledConn) close() {
	_ = c.netConn.Close()
}

// turnWaiter is one borrow queued on turnWaiters for an in-use turn past PoolSize.
type turnWaiter struct {
	// ready is closed exactly once, under inUseMu, by the give-back that grants the turn or by the
	// wait's own abandon; whichever runs first also removes the waiter from turnWaiters.
	ready chan struct{}
	// granted is written under inUseMu before close(ready), so reading it after <-ready is safe.
	granted bool
}

// liveCap is the frozen live-socket bound. After New it is turnCap.
func (sr *SimpleRedis) liveCap() int {
	if sr.turnCap != 0 {
		return sr.turnCap
	}
	if sr.poolSize > 0 {
		return sr.poolSize
	}
	return defaultPoolSize
}

// inUseTurnWait is how long borrow waits for a free in-use turn.
func (sr *SimpleRedis) inUseTurnWait() time.Duration {
	if sr.poolTimeout > 0 {
		return sr.poolTimeout
	}
	return defaultPoolTimeout
}

// ensureInUseTurns freezes the in-use-turn cap once from liveCap(), with every turn free. New calls this.
func (sr *SimpleRedis) ensureInUseTurns() {
	if sr.turnCap != 0 {
		return
	}
	liveCap := sr.liveCap()
	sr.turnCap = liveCap
	sr.freeTurns = liveCap
}

// freeInUseTurn returns one in-use turn to the pool. No-op before the pool is created.
// With a borrow queued, the turn goes to the oldest waiter in place, so a later borrow never overtakes it.
// An extra return when the semaphore is already full is dropped and counted on OverFrees so the caller does not hang.
// Lock and Unlock are explicit, without defer or a helper: under Yaegi each interpreted call or defer
// costs about a microsecond on every command, and nothing between them can panic.
func (sr *SimpleRedis) freeInUseTurn() {
	if sr.turnCap == 0 {
		return
	}
	overFree := false
	sr.inUseMu.Lock()
	if last := len(sr.turnWaiters) - 1; last >= 0 {
		oldest := sr.turnWaiters[0]
		copy(sr.turnWaiters, sr.turnWaiters[1:])
		sr.turnWaiters[last] = nil
		sr.turnWaiters = sr.turnWaiters[:last]
		oldest.granted = true
		close(oldest.ready)
	} else if sr.freeTurns < sr.turnCap {
		sr.freeTurns++
	} else {
		overFree = true
	}
	sr.inUseMu.Unlock()
	if overFree {
		// Over-free: some path returned a turn it did not take. Drop so the request does not hang.
		// No error exists on this path and none is invented for the log, so this stays a named event.
		sr.overFrees.Add(1)
		sr.logger.Error("simpleredis_over_free")
	}
}

// borrowSocket waits for an in-use turn, then takes an unused socket younger than idleTimeout, or dials.
// handshakeFailed is true only when a new dial's AUTH or SELECT failed after TCP succeeded.
//
// The unused list exists so a sequential burst on one client pays one dial plus one AUTH/SELECT instead of one per
// command. Its cost is that a peer restart, a failover, or CLIENT KILL of every accepted socket leaves the parked
// sockets dead while still younger than idleTimeout, so the idleTimeout sweep does not touch them and a borrow
// hands out a corpse.
//
// skipIdle is how a command escapes that vintage: this command already failed I/O on a socket it took from the
// unused list, so that list is no longer evidence the peer is up, and later attempts dial instead of popping the
// next corpse. Without it one command spends every attempt on dead sockets and returns redis:unreachable while the
// peer is healthy and accepting.
//
// go-redis instead refuses to hand out a dead socket at all: connCheck asks the fd directly (syscall.Conn to
// RawConn.Read to a non-blocking syscall.Read; EAGAIN means healthy, zero bytes means the peer is gone). That is
// closed to this client. Traefik registers syscall symbols only when useUnsafe is true in both the plugin manifest
// and the operator's static config, and manifest-true with operator-false makes Traefik refuse to load the plugin
// outright, which a middleware other people install cannot demand. The probe is also Unix-only, and Yaegi v0.16.1
// ignores //go:build lines, so go-redis's own conn_check.go / conn_check_dummy.go split silently resolves to the
// no-op. Detecting the dead socket by using it is what remains.
// See knowledge/research/ext_traefik_plugins_useunsafe/ and knowledge/research/ext_traefik_plugins_yaegi-build-constraints/.
//
//nolint:revive,stylecheck // error stays before handshakeFailed; reordering would collide with in-flight SimpleRedis PRs.
func (sr *SimpleRedis) borrowSocket(ctx context.Context, skipIdle bool) (conn *pooledConn, err error, handshakeFailed bool, fromIdle bool) {
	// closed is atomic; turnCap is written once in New before concurrent use.
	if sr.closed.Load() {
		sr.logger.Debug("borrowSocket: "+RedisUnreachable, "reason", "closed")
		return nil, errUnreachable, false, false
	}
	if sr.turnCap == 0 {
		return nil, errNotFromNew, false, false
	}
	if err := contextStop(ctx); err != nil {
		return nil, err, false, false
	}

	// No select here, or anywhere in session source (TestSessionSourceHasNoSelect): Yaegi v0.16.1 shares one
	// case list per select statement across goroutines, so concurrent borrows woke on each other's channels,
	// returned (nil, nil) and moved turns between clients (knowledge/research/ext_traefik_plugins_yaegi-select/).
	// One Lock either takes a free turn or queues this borrow, so a give-back never lets a later borrow
	// overtake a waiter. The uncontended take allocates nothing; it is inline for the reason freeInUseTurn gives.
	var waiter *turnWaiter
	sr.inUseMu.Lock()
	if sr.freeTurns > 0 {
		sr.freeTurns--
	} else {
		waiter = &turnWaiter{ready: make(chan struct{})}
		sr.turnWaiters = append(sr.turnWaiters, waiter)
	}
	sr.inUseMu.Unlock()
	if waiter != nil {
		// Waiter past poolSize: PoolTimeout, or ctx when the command deadline or a cancel comes sooner.
		// The abandon removes the waiter under the same lock a give-back grants under; whichever runs first
		// closes ready and the others find the waiter gone, so a turn is never lost or handed out twice,
		// and the timer and the ctx share it without a sync.Once. A time.AfterFunc plus a context.AfterFunc
		// measured cheaper than one context.AfterFunc on a context.WithTimeout child (compiled about
		// 350 vs 650 ns per wait, Yaegi about 4.1 vs 4.5 µs).
		abandon := func() {
			sr.inUseMu.Lock()
			for i, queued := range sr.turnWaiters {
				if queued == waiter {
					last := len(sr.turnWaiters) - 1
					copy(sr.turnWaiters[i:], sr.turnWaiters[i+1:])
					sr.turnWaiters[last] = nil
					sr.turnWaiters = sr.turnWaiters[:last]
					close(waiter.ready)
					break
				}
			}
			sr.inUseMu.Unlock()
		}
		poolTimer := time.AfterFunc(sr.inUseTurnWait(), abandon)
		stopCtxAbandon := context.AfterFunc(ctx, abandon)
		<-waiter.ready
		poolTimer.Stop()
		stopCtxAbandon()
		// A granted waiter continues even if ctx fired meanwhile: the turn is held like any other borrow's,
		// and the command path's own ctx checks end the command and release it.
		if !waiter.granted {
			// A fired context is not logged here: exec is the single owner of caller-cancel versus library-budget.
			if err := contextStop(ctx); err != nil {
				return nil, err, false, false
			}
			sr.logger.Warn("borrowSocket: "+RedisUnreachable, "reason", "pool_wait")
			return nil, errPoolWait, false, false
		}
	}

	// The turn is held from here. Return it on every path that does not hand a socket to the
	// caller, including a panic in takeIdleConn or dial.
	handedOff := false
	defer func() {
		if !handedOff {
			sr.freeInUseTurn()
		}
	}()

	if !skipIdle {
		// Prefer a young unused socket over a new dial.
		reused, stale, closed := sr.takeIdleConn()
		if closed {
			sr.logger.Debug("borrowSocket: "+RedisUnreachable, "reason", "closed")
			return nil, errUnreachable, false, false
		}
		for _, idle := range stale {
			idle.close()
		}
		// Sweeping expired sockets is pool hygiene, not a failure, so it stays a named event.
		if len(stale) > 0 {
			sr.logger.Debug("simpleredis_idle_swept")
		}
		if reused != nil {
			handedOff = true
			return reused, nil, false, true
		}
	} else if sr.closed.Load() {
		// Close raced after the turn was taken; do not dial a client that is shutting down.
		sr.logger.Debug("borrowSocket: "+RedisUnreachable, "reason", "closed")
		return nil, errUnreachable, false, false
	}
	// Idle miss, or skipIdle: dial while still holding the turn.
	conn, err, handshakeFailed = sr.dial(ctx)
	if err != nil {
		return nil, err, handshakeFailed, false
	}
	// A dial that worked is not a failure, so it stays a named event: reason separates a dial because the unused
	// list was empty from one that bypassed a list this command already found dead, and nothing else records
	// that the automatic recovery ran at all.
	if skipIdle {
		sr.logger.Debug("simpleredis_dial", "reason", "skip_idle")
	} else {
		sr.logger.Debug("simpleredis_dial", "reason", "idle_miss")
	}
	handedOff = true
	return conn, nil, false, false
}

// takeIdleConn sweeps unused sockets older than idleTimeout, then pops the newest survivor. Stale sockets are returned for close after the lock. closed is true when Close ran.
func (sr *SimpleRedis) takeIdleConn() (reused *pooledConn, stale []*pooledConn, closed bool) {
	sr.idleConnsMu.Lock()
	defer sr.idleConnsMu.Unlock()
	if sr.closed.Load() {
		return nil, nil, true
	}
	now := time.Now()
	// Keep still-young sockets in place; collect stale for close after unlock.
	survivors := sr.idleConns[:0]
	for _, conn := range sr.idleConns {
		if now.Sub(conn.lastUsed) < sr.idleTimeout {
			survivors = append(survivors, conn)
			continue
		}
		stale = append(stale, conn)
	}
	sr.idleConns = survivors
	// LIFO: reuse the newest survivor.
	if n := len(sr.idleConns); n > 0 {
		reused = sr.idleConns[n-1]
		sr.idleConns = sr.idleConns[:n-1]
	}
	return reused, stale, false
}

// release returns a clean conn to idleConns and frees the in-use turn, or closes it when dirty, closed, or idleConns is already at maxIdleConns.
func (sr *SimpleRedis) release(conn *pooledConn, reusable bool) {
	if reusable {
		// Stamp before park so a later close-because-full still recorded lastUsed.
		conn.lastUsed = time.Now()
	}
	if !reusable || !sr.parkIdleConn(conn) {
		conn.close()
	}
	sr.freeInUseTurn()
}

// parkIdleConn parks conn on idleConns when the client is open and unused sockets are under maxIdleConns. The idle mutex is released before return.
func (sr *SimpleRedis) parkIdleConn(conn *pooledConn) bool {
	sr.idleConnsMu.Lock()
	defer sr.idleConnsMu.Unlock()
	// Do not park when shut, or when unused sockets already equal the idle cap.
	if sr.closed.Load() || len(sr.idleConns) >= sr.maxIdleConns {
		return false
	}
	sr.idleConns = append(sr.idleConns, conn)
	return true
}

// dial opens TCP to host (plus a TLS client handshake when New stored a TLS configuration), then AUTH and SELECT
// when those New fields are set.
// Dialer.Timeout is the per-attempt cap on connect plus handshake; DialContext also honors ctx (overall budget or caller).
// handshakeFailed is true when the connection succeeded and AUTH or SELECT then failed; exec must not retry that error.
// A deterministic TLS rejection during the handshake is errTLS with handshakeFailed false: shouldRetry already
// refuses it because it is neither errUnreachable nor a retryable Redis reply.
//
//nolint:revive // error stays before handshakeFailed; reordering would collide with in-flight SimpleRedis PRs.
func (sr *SimpleRedis) dial(ctx context.Context) (conn *pooledConn, err error, handshakeFailed bool) {
	if err := contextStop(ctx); err != nil {
		return nil, err, false
	}
	netConn, err := sr.openNetConn(ctx)
	if err != nil {
		if stop := contextStop(ctx); stop != nil {
			return nil, stop, false
		}
		// A TLS rejection is an operator fix, so it is Error with the crypto/tls text, which is not a Redis reply.
		if isTLSRejection(err) {
			sr.logger.Error("dial: "+RedisTLS, "tls_error", err.Error())
			return nil, errTLS, false
		}
		// The dialer's own text is not published: the line is the unreachable sentinel, and host is on simpleredis_open.
		sr.logger.Debug("dial: " + RedisUnreachable)
		return nil, errUnreachable, false
	}
	conn = &pooledConn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		writer:  bufio.NewWriter(netConn),
	}

	// AUTH before SELECT so a passworded server accepts the session.
	// A handshake failure is Warn here and the verb says which step failed; do already logged what it saw.
	// Never log the peer's text: Redis 7.4 AUTH replies can quote Config.Pass.
	if sr.user != "" || sr.pass != "" {
		if _, _, err = sr.do(ctx, conn, sr.authArgs()); err != nil {
			conn.close()
			switch {
			case errors.Is(err, ErrNoAuth):
				sr.logger.Warn("dial: "+RedisNoAuth, "verb", verbAuth)
			case errors.Is(err, ErrTLS):
				sr.logger.Warn("dial: "+RedisTLS, "verb", verbAuth)
			default:
				sr.logger.Warn("dial: redis error", "verb", verbAuth)
			}
			return nil, err, true
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte(verbSelect), []byte(sr.database)}); err != nil {
			conn.close()
			switch {
			case errors.Is(err, ErrNoAuth):
				sr.logger.Warn("dial: "+RedisNoAuth, "verb", verbSelect)
			case errors.Is(err, ErrTLS):
				sr.logger.Warn("dial: "+RedisTLS, "verb", verbSelect)
			default:
				sr.logger.Warn("dial: redis error", "verb", verbSelect)
			}
			return nil, err, true
		}
	}
	return conn, nil, false
}

// openNetConn obtains the socket for one dial attempt: plain TCP when New stored no TLS configuration, else TCP
// plus a TLS client handshake with that configuration. Both are bounded by DialTimeout and ctx.
// tls.Dialer, not tls.DialWithDialer: only DialContext honours ctx during the handshake. ServerName is not built
// from host here; crypto/tls takes it from the address when the configuration leaves it empty.
func (sr *SimpleRedis) openNetConn(ctx context.Context) (net.Conn, error) {
	if sr.tlsConfig == nil {
		dialer := net.Dialer{Timeout: sr.DialTimeout()}
		return dialer.DialContext(ctx, "tcp", sr.host)
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: sr.DialTimeout()}, Config: sr.tlsConfig}
	return dialer.DialContext(ctx, "tcp", sr.host)
}

// authArgs is the AUTH command for this client: AUTH <user> <pass> when an ACL user is set (also with an empty
// pass), else AUTH <pass>. Redis reads the one-argument form as the default user.
func (sr *SimpleRedis) authArgs() [][]byte {
	if sr.user != "" {
		return [][]byte{[]byte(verbAuth), []byte(sr.user), []byte(sr.pass)}
	}
	return [][]byte{[]byte(verbAuth), []byte(sr.pass)}
}
