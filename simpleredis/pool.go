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
		overFrees := sr.overFrees.Add(1)
		sr.logError(MsgOverFree, "over_frees", overFrees, "turns", len(sr.inUseTurns), "cap", cap(sr.inUseTurns))
	}
}

// borrow waits for an in-use turn, then takes an unused socket younger than idleTimeout, or dials.
// handshakeFailed is true only when a new dial's AUTH or SELECT failed after TCP succeeded.
//
//nolint:revive // error stays before handshakeFailed; reordering would collide with in-flight SimpleRedis PRs.
func (sr *SimpleRedis) borrow(ctx context.Context) (conn *pooledConn, err error, handshakeFailed bool) {
	// closed is atomic; inUseTurns is written once in New before concurrent use.
	if sr.closed.Load() {
		return nil, errUnreachable, false
	}
	if sr.inUseTurns == nil {
		sr.logError(MsgNotFromNew, "host", sr.host)
		return nil, errNotFromNew, false
	}
	if err := contextStop(ctx); err != nil {
		return nil, err, false
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
			if errorsIsCanceled(ctx.Err()) {
				sr.logDebugCanceled(ctx)
			}
			return nil, ctx.Err(), false
		case <-timer.C:
			if err := contextStop(ctx); err != nil {
				if errorsIsCanceled(err) {
					sr.logDebugCanceled(ctx)
				}
				return nil, err, false
			}
			sr.logWarn(MsgPoolExhausted, "pool_size", sr.liveCap(), "wait", sr.inUseTurnWait(), "host", sr.host)
			return nil, errPoolWait, false
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

	// Prefer a young unused socket over a new dial.
	reused, stale, closed := sr.takeIdleConn()
	if closed {
		return nil, errUnreachable, false
	}
	for _, conn := range stale {
		conn.close()
	}
	if reused != nil {
		handedOff = true
		return reused, nil, false
	}
	// Idle miss: dial while still holding the turn.
	dialReason := dialReasonIdleMiss
	if len(stale) > 0 {
		dialReason = dialReasonStale
	}
	conn, err, handshakeFailed = sr.dial(ctx)
	if err != nil {
		return nil, err, handshakeFailed
	}
	sr.logDebugDial(ctx, dialReason)
	handedOff = true
	return conn, nil, false
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
	if swept := len(stale); swept > 0 {
		sr.logDebugIdleSwept(context.Background(), swept, len(sr.idleConns))
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
	if sr.closed.Load() {
		return false
	}
	if len(sr.idleConns) >= sr.maxIdleConns {
		sr.logDebugSocketClosed(context.Background(), closedReasonIdleCap)
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
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte(cmdAuth), []byte(sr.pass)}); err != nil {
			conn.close()
			if err != errNoAuth { //nolint:errorlint // AUTH-class already logged in do
				sr.logWarn(MsgHandshakeFailed, "error", err, "host", sr.host)
			}
			return nil, err, true
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte(cmdSelect), []byte(sr.database)}); err != nil {
			conn.close()
			if err != errNoAuth { //nolint:errorlint // AUTH-class already logged in do
				sr.logWarn(MsgHandshakeFailed, "error", err, "host", sr.host)
			}
			return nil, err, true
		}
	}
	return conn, nil, false
}
