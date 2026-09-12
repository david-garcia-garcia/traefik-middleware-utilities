// Package simpleredis is a stdlib pooled TCP RESP client (GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL, MSetEX, MSetEXAt).
package simpleredis

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Error strings for redis.
const (
	RedisUnreachable = "redis:unreachable"
	RedisMiss        = "redis:miss"
	RedisTimeout     = "redis:timeout"
	RedisNoAuth      = "redis:noauth"
	RedisIssue       = "redis:issue?"
)

var (
	errUnreachable = errors.New(RedisUnreachable)
	// errPoolWait is a waiter past liveCap. Error() is redis:unreachable so callers still match that token. Distinct from errUnreachable so MaxRetries does not multiply poolTimeout.
	errPoolWait = errors.New(RedisUnreachable)
	// errNotFromNew is a client that did not come from New (nil in-use-turn channel). Error() is redis:unreachable so callers still match that token. Distinct from errUnreachable so MaxRetries does not sleep a programming error.
	errNotFromNew = errors.New(RedisUnreachable)
	errMiss       = errors.New(RedisMiss)
	errTimeout    = errors.New(RedisTimeout)
	errNoAuth     = errors.New(RedisNoAuth)
	errIssue      = errors.New(RedisIssue)
)

// SimpleRedis is a pooled TCP RESP client. Obtain one with New; commands dial on first use.
// Pool, timeout, and retry knobs live on Config and are frozen at New; they are not fields on this type.
type SimpleRedis struct {
	host     string
	pass     string
	database string

	maxRetries      int
	minRetryBackoff time.Duration
	maxRetryBackoff time.Duration

	poolSize     int
	maxIdleConns int
	poolTimeout  time.Duration
	idleTimeout  time.Duration
	dialTimeout  time.Duration
	ioTimeout    time.Duration

	// idleConnsMu guards idleConns (the unused sockets waiting for reuse).
	idleConnsMu sync.Mutex
	idleConns   []*pooledConn
	closed      atomic.Bool
	// inUseTurns is a PoolSize-buffered semaphore of concurrent in-use sockets. Idle sockets do not hold a turn.
	inUseTurns chan struct{}

	// groupWriteMu guards groupWrite (native MSETEX vs Lua fallback). Not idleConnsMu: that lock is the unused-socket list.
	groupWriteMu sync.Mutex
	groupWrite   groupWritePath
}

// New copies cfg onto a client and builds the in-use-turn channel. Does not dial. Call before concurrent use.
func New(cfg Config) *SimpleRedis {
	cfg = cfg.applyDefaults()
	sr := &SimpleRedis{
		host:            cfg.Host,
		pass:            cfg.Pass,
		database:        cfg.Database,
		maxRetries:      cfg.MaxRetries,
		minRetryBackoff: cfg.MinRetryBackoff,
		maxRetryBackoff: cfg.MaxRetryBackoff,
		poolSize:        cfg.PoolSize,
		maxIdleConns:    cfg.MaxIdleConns,
		poolTimeout:     cfg.PoolTimeout,
		idleTimeout:     cfg.IdleTimeout,
		dialTimeout:     cfg.DialTimeout,
		ioTimeout:       cfg.IOTimeout,
	}
	sr.ensureInUseTurns()
	return sr
}

// Close drains unused pooled connections and stops pooling. Further Get/Set/Del/MGet/Incr/IncrBy/Expire/ExpireAt/Eval/MSetEX/MSetEXAt return redis:unreachable and do not dial. In-flight commands still finish; their sockets are closed on release. Safe to call more than once.
func (sr *SimpleRedis) Close() {
	if !sr.closed.CompareAndSwap(false, true) {
		return
	}
	sr.idleConnsMu.Lock()
	idleConns := sr.idleConns
	sr.idleConns = nil
	sr.idleConnsMu.Unlock()
	for _, conn := range idleConns {
		conn.close()
	}
}

// isClosed is true after Close. Used so a closed-client unreachable does not spin MaxRetries.
func (sr *SimpleRedis) isClosed() bool {
	return sr.closed.Load()
}

// PoolSize is the live-socket cap New froze (idle plus in-use).
func (sr *SimpleRedis) PoolSize() int {
	return sr.liveCap()
}

// MaxIdleConns is the idle-list trim New froze.
func (sr *SimpleRedis) MaxIdleConns() int {
	return sr.maxIdleConns
}

// PoolTimeout is how long a waiter past PoolSize blocks.
func (sr *SimpleRedis) PoolTimeout() time.Duration {
	return sr.inUseTurnWait()
}

// IdleTimeout is the idle reuse gate New froze.
func (sr *SimpleRedis) IdleTimeout() time.Duration {
	if sr.idleTimeout > 0 {
		return sr.idleTimeout
	}
	return defaultIdleTimeout
}

// DialTimeout is the TCP dial bound New froze.
func (sr *SimpleRedis) DialTimeout() time.Duration {
	if sr.dialTimeout > 0 {
		return sr.dialTimeout
	}
	return defaultDialTimeout
}

// IOTimeout is the per-command deadline New froze.
func (sr *SimpleRedis) IOTimeout() time.Duration {
	if sr.ioTimeout > 0 {
		return sr.ioTimeout
	}
	return defaultIOTimeout
}

// MaxRetries is the retry sentinel New froze (0 default, -1 off).
func (sr *SimpleRedis) MaxRetries() int {
	return sr.maxRetries
}
