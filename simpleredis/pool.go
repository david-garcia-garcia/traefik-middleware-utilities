package simpleredis

import (
	"bufio"
	"context"
	"net"
	"time"
)

// pooledConn is one TCP socket plus RESP reader/writer kept in idleConns.
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

// liveCap is the frozen live-socket bound. After New it is cap(inUseTurns).
func (sr *SimpleRedis) liveCap() int {
	if sr.inUseTurns != nil {
		return cap(sr.inUseTurns)
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

// ensureInUseTurns creates the in-use-turn channel once from liveCap(). New calls this.
func (sr *SimpleRedis) ensureInUseTurns() {
	if sr.inUseTurns != nil {
		return
	}
	liveCap := sr.liveCap()
	sr.inUseTurns = make(chan struct{}, liveCap)
	for i := 0; i < liveCap; i++ {
		sr.inUseTurns <- struct{}{}
	}
}

// freeInUseTurn returns one in-use-socket token to the pool. No-op before the pool is created.
// An extra return when the semaphore is already full is dropped and counted on OverFrees so the caller does not hang.
func (sr *SimpleRedis) freeInUseTurn() {
	if sr.inUseTurns == nil {
		return
	}
	select {
	case sr.inUseTurns <- struct{}{}:
	default:
		// Over-free: some path returned a turn it did not take. Drop so the request does not hang.
		sr.overFrees.Add(1)
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
	// closed is atomic; inUseTurns is written once in New before concurrent use.
	if sr.closed.Load() {
		return nil, errUnreachable, false, false
	}
	if sr.inUseTurns == nil {
		return nil, errNotFromNew, false, false
	}
	if err := contextStop(ctx); err != nil {
		return nil, err, false, false
	}

	// Uncontended borrow must not allocate a timer; the wait exists only for a waiter past poolSize.
	select {
	case <-sr.inUseTurns:
	default:
		// Waiter past poolSize: PoolTimeout timer, or ctx.Done when the command deadline is sooner.
		timer := time.NewTimer(sr.inUseTurnWait())
		select {
		case <-sr.inUseTurns:
			if !timer.Stop() {
				<-timer.C
			}
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err(), false, false
		case <-timer.C:
			if err := contextStop(ctx); err != nil {
				return nil, err, false, false
			}
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
			return nil, errUnreachable, false, false
		}
		for _, idle := range stale {
			idle.close()
		}
		if reused != nil {
			handedOff = true
			return reused, nil, false, true
		}
	} else if sr.closed.Load() {
		// Close raced after the turn was taken; do not dial a client that is shutting down.
		return nil, errUnreachable, false, false
	}
	// Idle miss, or skipIdle: dial while still holding the turn.
	conn, err, handshakeFailed = sr.dial(ctx)
	if err != nil {
		return nil, err, handshakeFailed, false
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

// dial opens TCP to host, then AUTH and SELECT when those New fields are set.
// Dialer.Timeout is the per-attempt cap; DialContext also honors ctx (overall budget or caller).
// handshakeFailed is true when TCP succeeded and AUTH or SELECT then failed; exec must not retry that error.
//
//nolint:revive // error stays before handshakeFailed; reordering would collide with in-flight SimpleRedis PRs.
func (sr *SimpleRedis) dial(ctx context.Context) (conn *pooledConn, err error, handshakeFailed bool) {
	if err := contextStop(ctx); err != nil {
		return nil, err, false
	}
	dialer := net.Dialer{Timeout: sr.DialTimeout()}
	netConn, err := dialer.DialContext(ctx, "tcp", sr.host)
	if err != nil {
		if stop := contextStop(ctx); stop != nil {
			return nil, stop, false
		}
		return nil, errUnreachable, false
	}
	conn = &pooledConn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		writer:  bufio.NewWriter(netConn),
	}

	// AUTH before SELECT so a passworded server accepts the session.
	if sr.pass != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("AUTH"), []byte(sr.pass)}); err != nil {
			conn.close()
			return nil, err, true
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("SELECT"), []byte(sr.database)}); err != nil {
			conn.close()
			return nil, err, true
		}
	}
	return conn, nil, false
}
