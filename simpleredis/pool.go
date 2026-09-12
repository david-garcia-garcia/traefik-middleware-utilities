package simpleredis

import (
	"bufio"
	"net"
	"time"
)

// pooledConn is one TCP socket plus RESP reader and encode scratch kept in idleConns.
type pooledConn struct {
	netConn  net.Conn
	reader   *bufio.Reader
	buf      []byte
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
func (sr *SimpleRedis) freeInUseTurn() {
	if sr.inUseTurns == nil {
		return
	}
	sr.inUseTurns <- struct{}{}
}

// borrow waits for an in-use turn, then takes an unused socket younger than idleTimeout, or dials.
func (sr *SimpleRedis) borrow() (*pooledConn, error) {
	// closed is atomic; inUseTurns is written once in New before concurrent use.
	if sr.closed.Load() {
		return nil, errUnreachable
	}
	if sr.inUseTurns == nil {
		return nil, errUnreachable
	}

	// Uncontended borrow must not allocate a timer; the wait exists only for a waiter past poolSize.
	select {
	case <-sr.inUseTurns:
	default:
		// Waiter past poolSize: allocate a stoppable timer (not time.After).
		timer := time.NewTimer(sr.inUseTurnWait())
		select {
		case <-sr.inUseTurns:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			return nil, errPoolWait
		}
	}

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
	// Idle miss: dial while still holding the turn.
	conn, err := sr.dial()
	if err != nil {
		sr.freeInUseTurn()
		return nil, err
	}
	return conn, nil
}

// takeIdleConn pops unused sockets until one is younger than idleTimeout. Stale sockets are returned for close after the lock. closed is true when Close ran.
func (sr *SimpleRedis) takeIdleConn() (reused *pooledConn, stale []*pooledConn, closed bool) {
	sr.idleConnsMu.Lock()
	defer sr.idleConnsMu.Unlock()
	if sr.closed.Load() {
		return nil, nil, true
	}
	now := time.Now()
	for len(sr.idleConns) > 0 {
		conn := sr.idleConns[len(sr.idleConns)-1]
		sr.idleConns = sr.idleConns[:len(sr.idleConns)-1]
		if now.Sub(conn.lastUsed) < sr.idleTimeout {
			return conn, stale, false
		}
		stale = append(stale, conn)
	}
	return nil, stale, false
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
	// Drop a huge encode scratch so one large SET cannot pin that idle conn.
	if cap(conn.buf) > maxIdleEncodeBuf {
		conn.buf = nil
	}
	sr.idleConns = append(sr.idleConns, conn)
	sr.idleConnsMu.Unlock()
	sr.freeInUseTurn()
}

// dial opens TCP to host, then AUTH and SELECT when those New fields are set.
func (sr *SimpleRedis) dial() (*pooledConn, error) {
	dialer := net.Dialer{Timeout: sr.dialTimeout}
	netConn, err := dialer.Dial("tcp", sr.host)
	if err != nil {
		return nil, errUnreachable
	}
	conn := &pooledConn{
		netConn: netConn,
		reader:  bufio.NewReader(netConn),
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
