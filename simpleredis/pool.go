package simpleredis

import (
	"bufio"
	"context"
	"net"
	"time"
)

// handshakeFailure is an AUTH or SELECT error from dial. exec must not retry it.
type handshakeFailure struct {
	err error
}

// Error is the inner AUTH or SELECT failure text (redis:unreachable, LOADING …, redis:noauth).
func (e handshakeFailure) Error() string { return e.err.Error() }

// Unwrap is the inner AUTH or SELECT failure.
func (e handshakeFailure) Unwrap() error { return e.err }

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

// borrow waits for an in-use turn, then takes an unused socket younger than idleTimeout, or dials.
func (sr *SimpleRedis) borrow(ctx context.Context) (*pooledConn, error) {
	// closed is atomic; inUseTurns is written once in New before concurrent use.
	if sr.closed.Load() {
		return nil, errUnreachable
	}
	if sr.inUseTurns == nil {
		return nil, errNotFromNew
	}
	if err := contextStop(ctx); err != nil {
		return nil, err
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
			return nil, ctx.Err()
		case <-timer.C:
			if err := contextStop(ctx); err != nil {
				return nil, err
			}
			// Timed-out waiter: restore leaked turns if nothing is owned, take one, then
			// idle-or-dial while still holding turnRecoverMu so another waiter cannot refill and dial past PoolSize.
			return sr.borrowAfterPoolWait(ctx)
		}
	}

	return sr.takeIdleOrDial(ctx)
}

// borrowAfterPoolWait restores leaked turns if the client owns no socket, takes one turn, then
// idle-or-dials. turnRecoverMu is held until this borrow returns so concurrent waiters cannot mint extra turns.
func (sr *SimpleRedis) borrowAfterPoolWait(ctx context.Context) (*pooledConn, error) {
	sr.turnRecoverMu.Lock()
	defer sr.turnRecoverMu.Unlock()
	if sr.heldSockets.Load() == 0 {
		sr.recoverLostTurnsLocked()
	}
	select {
	case <-sr.inUseTurns:
	default:
		return nil, errPoolWait
	}
	return sr.takeIdleOrDial(ctx)
}

// recoverLostTurnsLocked refills inUseTurns when idle is empty and heldSockets is 0.
// Caller holds turnRecoverMu.
func (sr *SimpleRedis) recoverLostTurnsLocked() {
	if sr.inUseTurns == nil || sr.closed.Load() {
		return
	}
	if sr.heldSockets.Load() != 0 {
		return
	}
	sr.idleConnsMu.Lock()
	idleCount := len(sr.idleConns)
	sr.idleConnsMu.Unlock()
	if idleCount != 0 {
		return
	}
	filled := 0
	for {
		select {
		case sr.inUseTurns <- struct{}{}:
			filled++
		default:
			if filled > 0 {
				sr.lostTurns.Add(int64(filled))
			}
			return
		}
	}
}

// takeIdleOrDial pops a young unused socket or dials while the caller already holds a turn.
func (sr *SimpleRedis) takeIdleOrDial(ctx context.Context) (*pooledConn, error) {
	// Prefer a young unused socket over a new dial.
	reused, stale, closed := sr.takeIdleConn()
	if closed {
		sr.freeInUseTurn()
		return nil, errUnreachable
	}
	for _, conn := range stale {
		conn.close()
	}
	if reused != nil {
		return reused, nil
	}
	// Idle miss: dial while still holding the turn. Defer restores heldSockets if AUTH/SELECT panics.
	sr.heldSockets.Add(1)
	defer sr.heldSockets.Add(-1)
	conn, err := sr.dial(ctx)
	if err != nil {
		sr.freeInUseTurn()
		return nil, err
	}
	return conn, nil
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

// release returns a clean conn to idleConns and frees the in-use turn, or closes it when dirty, closed, or idleConns is full at the live cap.
func (sr *SimpleRedis) release(conn *pooledConn, reusable bool) {
	if !reusable {
		conn.close()
		sr.freeInUseTurn()
		return
	}
	conn.lastUsed = time.Now()

	sr.idleConnsMu.Lock()
	// Close only when shut or idleConns is already maxIdleConns and live is at liveCap().
	// inUse still includes this socket until freeInUseTurn runs.
	idleConnsFull := len(sr.idleConns) >= sr.maxIdleConns
	inUse := 0
	if sr.inUseTurns != nil {
		inUse = sr.liveCap() - len(sr.inUseTurns)
	}
	live := len(sr.idleConns) + inUse
	if sr.closed.Load() || (idleConnsFull && live >= sr.liveCap()) {
		sr.idleConnsMu.Unlock()
		conn.close()
		sr.freeInUseTurn()
		return
	}
	sr.idleConns = append(sr.idleConns, conn)
	sr.idleConnsMu.Unlock()
	sr.freeInUseTurn()
}

// dial opens TCP to host, then AUTH and SELECT when those New fields are set.
// Dialer.Timeout is the per-attempt cap; DialContext also honors ctx (overall budget or caller).
func (sr *SimpleRedis) dial(ctx context.Context) (*pooledConn, error) {
	if err := contextStop(ctx); err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: sr.DialTimeout()}
	netConn, err := dialer.DialContext(ctx, "tcp", sr.host)
	if err != nil {
		if stop := contextStop(ctx); stop != nil {
			return nil, stop
		}
		return nil, errUnreachable
	}
	conn := &pooledConn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
		writer:  bufio.NewWriter(netConn),
	}

	// AUTH before SELECT so a passworded server accepts the session.
	if sr.pass != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("AUTH"), []byte(sr.pass)}); err != nil {
			conn.close()
			return nil, handshakeFailure{err: err}
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("SELECT"), []byte(sr.database)}); err != nil {
			conn.close()
			return nil, handshakeFailure{err: err}
		}
	}
	return conn, nil
}
