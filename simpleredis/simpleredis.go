// Package simpleredis is a stdlib pooled TCP RESP client (GET, MGET, SET with EX, DEL, INCR, INCRBY, EXPIRE, EXPIREAT, EVAL, MSetEX, MSetEXAt).
package simpleredis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
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

	poolSize       int
	maxIdleConns   int
	poolTimeout    time.Duration
	idleTimeout    time.Duration
	dialTimeout    time.Duration
	commandTimeout time.Duration
	logger         *slog.Logger

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
		commandTimeout:  cfg.CommandTimeout,
		logger:          cfg.Logger,
	}
	// Nil Config.Logger is a discard handler so later call sites never nil-check.
	if sr.logger == nil {
		sr.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	sr.ensureInUseTurns()
	sr.logger.Debug("simpleredis_open", "host", sr.host)
	return sr, nil
}

// Log sites: the function that saw a failure. Named once so the same literal is not repeated at every emit.
const (
	siteBorrow = "borrowSocket"
	siteDial   = "dial"
	siteDo     = "do"
	siteExec   = "exec"
)

// slog attribute keys, and the reason values that separate two paths that share one site and one sentinel.
const (
	attrReason = "reason"
	attrVerb   = "verb"

	reasonClosed            = "closed"
	reasonNotFromNew        = "not_from_new"
	reasonPoolWait          = "pool_wait"
	reasonIdleMiss          = "idle_miss"
	reasonSkipIdle          = "skip_idle"
	reasonSetDeadline       = "set_deadline"
	reasonUnreadBeforeWrite = "unread_before_write"
	reasonWrite             = "write"
	reasonRead              = "read"
)

// logSite records a failure at the function that saw it, as the slog message "<site>: <cause>".
//
// It logs and returns nothing. It MUST NOT wrap the error it is given. RedisUnreachable, RedisTimeout,
// RedisNoAuth, RedisIssue, RedisMiss and RedisUnsupportedReply are exported text that windowcounter,
// tokenbucket, leakybucket and e2e/simpleredisprobe compare against, and std_go_simpleredis_tcp-session pins
// those exact strings in its requirements; a "<site>: <cause>" wrap would change every one of them and force
// each surviving err.Error() prefix check through an unwrap helper. The site is for the operator, not the caller.
//
// Why a site at all: six paths return the one redis:unreachable text (a TCP dial that failed, the pre-write
// and the SetDeadline refusals in do, a short read in do, a closed client, and a client that never came from
// New). The sentinel alone cannot tell an operator which of them happened.
func (sr *SimpleRedis) logSite(level slog.Level, site string, err error, attrs ...any) {
	if err == nil {
		return
	}
	// A client that did not come from New has no logger; its first command still reports errNotFromNew here.
	if sr.logger == nil {
		return
	}
	sr.logger.Log(context.Background(), level, site+": "+loggableCause(err), attrs...)
}

// loggableCause is the text logSite may publish for err.
//
// An error this package owns prints itself. Anything else came from the peer and prints its leading error
// code only, because a Redis error reply carries text this client must never republish:
//
//   - Redis 7.4 answers AUTH against a nopass default user with "ERR AUTH <password> called without any
//     password configured for the default user". That is not an AUTH-class prefix, so replyError does not map
//     it to redis:noauth and nothing else keeps Config.Pass off the line.
//   - An unknown verb is refused with the command's own arguments quoted back ("ERR unknown command 'MSETEX',
//     with args beginning with: '2', '<key>', '<value>'"), which is Redis key names and values.
//
// Trimming at this one sink is why no call site has to decide: the rule holds for every site, level and verb.
// The caller still receives the peer's full text; only the log line is trimmed.
func loggableCause(err error) string {
	switch {
	case errors.Is(err, ErrUnreachable), errors.Is(err, ErrTimeout), errors.Is(err, ErrNoAuth),
		errors.Is(err, ErrMiss), errors.Is(err, ErrIssue), errors.Is(err, ErrUnsupportedReply),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err.Error()
	}
	return peerErrorCode(err.Error())
}

// peerErrorCode is the leading error code of a Redis error reply (ERR, LOADING, MISCONF, NOSCRIPT, WRONGTYPE)
// and no byte after it. A first token that is not all A-Z is not an error code and is not published at all.
func peerErrorCode(text string) string {
	code := text
	if space := strings.IndexByte(text, ' '); space >= 0 {
		code = text[:space]
	}
	if code == "" {
		return unnamedPeerReply
	}
	for i := 0; i < len(code); i++ {
		if code[i] < 'A' || code[i] > 'Z' {
			return unnamedPeerReply
		}
	}
	return code
}

// unnamedPeerReply stands in for a peer error whose leading token is not a Redis error code.
const unnamedPeerReply = "redis error reply"

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

// DialTimeout is the per-attempt TCP dial bound New froze.
func (sr *SimpleRedis) DialTimeout() time.Duration {
	if sr.dialTimeout > 0 {
		return sr.dialTimeout
	}
	return defaultDialTimeout
}

// CommandTimeout is the whole-command bound New froze: every attempt, dial, AUTH, SELECT and command I/O share it.
func (sr *SimpleRedis) CommandTimeout() time.Duration {
	if sr.commandTimeout > 0 {
		return sr.commandTimeout
	}
	return defaultCommandTimeout
}

// MaxRetries is the extra-retry count New froze (1 after zero Config, -1 off). It bounds attempts, not wall time.
func (sr *SimpleRedis) MaxRetries() int {
	return sr.maxRetries
}
