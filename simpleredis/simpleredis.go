// Package simpleredis is a stdlib pooled TCP RESP client (GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL, MSetEX, MSetEXAt).
package simpleredis

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Error strings for redis. Keep for display and legacy text matching.
const (
	RedisUnreachable      = "redis:unreachable"
	RedisMiss             = "redis:miss"
	RedisTimeout          = "redis:timeout"
	RedisNoAuth           = "redis:noauth"
	RedisIssue            = "redis:issue?"
	RedisUnsupportedReply = "redis:unsupported-reply"
)

// Exported sentinels. Match with errors.Is or IsMiss / IsUnreachable / IsPoolWait, not string equality.
var (
	ErrUnreachable      = errors.New(RedisUnreachable)
	ErrMiss             = errors.New(RedisMiss)
	ErrTimeout          = errors.New(RedisTimeout)
	ErrNoAuth           = errors.New(RedisNoAuth)
	ErrIssue            = errors.New(RedisIssue)
	ErrUnsupportedReply = errors.New(RedisUnsupportedReply)
)

// ErrPoolWait is pool saturation. It wraps ErrUnreachable so callers matching the broad condition keep working.
var ErrPoolWait = fmt.Errorf("%w", ErrUnreachable)

var (
	errUnreachable      = ErrUnreachable
	errPoolWait         = ErrPoolWait
	errMiss             = ErrMiss
	errTimeout          = ErrTimeout
	errNoAuth           = ErrNoAuth
	errIssue            = ErrIssue
	errUnsupportedReply = ErrUnsupportedReply
	// errNotFromNew is a client that did not come from New (nil in-use-turn channel). Error() is redis:unreachable. Distinct from errUnreachable so MaxRetries does not sleep a programming error. Wraps ErrUnreachable so IsUnreachable stays true.
	errNotFromNew = fmt.Errorf("%w", ErrUnreachable)
)

// IsMiss reports whether err is a Redis miss, including wrapping.
func IsMiss(err error) bool { return errors.Is(err, ErrMiss) }

// IsUnreachable reports whether err is redis:unreachable, including pool wait and wrapping.
func IsUnreachable(err error) bool { return errors.Is(err, ErrUnreachable) }

// IsPoolWait reports whether err is pool saturation, including wrapping.
func IsPoolWait(err error) bool { return errors.Is(err, ErrPoolWait) }

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
	// overFrees counts in-use-turn returns that were dropped because the semaphore was already full.
	overFrees atomic.Int64

	// groupWriteMu guards groupWrite (native MSETEX vs Lua fallback). Not idleConnsMu: that lock is the unused-socket list.
	groupWriteMu sync.Mutex
	groupWrite   groupWritePath
}

// New copies cfg onto a client and builds the in-use-turn channel. Does not dial. Call before concurrent use.
// An explicit MaxIdleConns above PoolSize returns ErrMaxIdleConnsAbovePoolSize and a nil client; a defaulted one follows PoolSize.
func New(cfg Config) (*SimpleRedis, error) {
	cfg, err := cfg.applyDefaults()
	if err != nil {
		return nil, err
	}
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
	return sr, nil
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

// MaxIdleConns is the idle-list trim New froze, never above PoolSize. New does not create a client when an explicit trim sits above PoolSize.
func (sr *SimpleRedis) MaxIdleConns() int {
	return sr.maxIdleConns
}

// OverFrees is how many extra in-use-turn returns were dropped because the semaphore was already full.
func (sr *SimpleRedis) OverFrees() int64 {
	return sr.overFrees.Load()
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

// IOTimeout is the stall bound New froze (quiet time between socket reads/writes).
func (sr *SimpleRedis) IOTimeout() time.Duration {
	if sr.ioTimeout > 0 {
		return sr.ioTimeout
	}
	return defaultIOTimeout
}

// MaxRetries is the extra-retry count New froze (1 after zero Config, -1 off).
func (sr *SimpleRedis) MaxRetries() int {
	return sr.maxRetries
}
