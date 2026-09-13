package simpleredis

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	// MsgPanic is recovered in runOnConn, then re-panicked.
	MsgPanic = "simpleredis_panic"
	// MsgNoAuth is AUTH rejected (WRONGPASS / NOAUTH / NOPERM / ERR Client sent AUTH).
	MsgNoAuth = "simpleredis_noauth"
	// MsgNotFromNew is a command on a client that was not built with New.
	MsgNotFromNew = "simpleredis_not_from_new"
	// MsgOverFree is an extra freeInUseTurn with no matching borrow.
	MsgOverFree = "simpleredis_over_free"
	// MsgSocketPoisoned is leftover RESP after a complete reply; the socket is destroyed.
	MsgSocketPoisoned = "simpleredis_socket_poisoned"
	// MsgAuthLeftover is a non-empty reader before AUTH or SELECT is written.
	MsgAuthLeftover = "simpleredis_auth_leftover"
	// MsgPoolExhausted is a waiter that exceeded PoolTimeout.
	MsgPoolExhausted = "simpleredis_pool_exhausted"
	// MsgShortBulk is a peer that announced more bulk payload than it wrote.
	MsgShortBulk = "simpleredis_short_bulk"
	// MsgBadReply is malformed or unsupported RESP.
	MsgBadReply = "simpleredis_bad_reply"
	// MsgHandshakeFailed is AUTH or SELECT failed for a non-auth reason.
	MsgHandshakeFailed = "simpleredis_handshake_failed"
	// MsgDial is a new TCP socket.
	MsgDial = "simpleredis_dial"
	// MsgIdleSwept is stale idle sockets closed on borrow.
	MsgIdleSwept = "simpleredis_idle_swept"
	// MsgRetry is a retry attempt about to sleep.
	MsgRetry = "simpleredis_retry"
	// MsgTimeout is an I/O deadline or overall command budget elapsed.
	MsgTimeout = "simpleredis_timeout"
	// MsgCanceled is the caller context cancelled mid-command.
	MsgCanceled = "simpleredis_canceled"
	// MsgSocketClosed is a socket destroyed for a benign reason.
	MsgSocketClosed = "simpleredis_socket_closed"
	// MsgCapability is MSetEX resolving native vs Lua.
	MsgCapability = "simpleredis_capability"
	// MsgNoScript is EVAL NOSCRIPT triggering a script reload.
	MsgNoScript = "simpleredis_noscript"
	// MsgOpen is New, with frozen knobs (never Pass).
	MsgOpen = "simpleredis_open"
	// MsgClose is Close after idle sockets are drained.
	MsgClose = "simpleredis_close"
)

const (
	dialReasonIdleMiss  = "idle_miss"
	dialReasonStale     = "stale"
	closedReasonCancel  = "cancel"
	closedReasonIdleCap = "idle_cap"
	capabilityNative    = "native"
	capabilityLua       = "lua"
)

// logError emits an Error line when a logger was frozen at New.
func (sr *SimpleRedis) logError(msg string, args ...any) {
	if sr == nil || sr.logger == nil {
		return
	}
	sr.logger.Error(msg, args...)
}

// logWarn emits a Warn line when a logger was frozen at New.
func (sr *SimpleRedis) logWarn(msg string, args ...any) {
	if sr == nil || sr.logger == nil {
		return
	}
	sr.logger.Warn(msg, args...)
}

// debugLogger is the frozen logger when Debug is enabled. Callers MUST NOT build
// attributes unless ok is true, so a nil or Warn-only logger allocates nothing.
func (sr *SimpleRedis) debugLogger(ctx context.Context) (logger *slog.Logger, ok bool) {
	if sr == nil || sr.logger == nil {
		return nil, false
	}
	if !sr.logger.Enabled(ctx, slog.LevelDebug) {
		return nil, false
	}
	return sr.logger, true
}

// logDebugTimeout is simpleredis_timeout. Typed so the Enabled check runs before attrs exist.
func (sr *SimpleRedis) logDebugTimeout(ctx context.Context, ioErr error) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgTimeout, "error", ioErr, "host", sr.host)
	}
}

// logDebugCanceled is simpleredis_canceled.
func (sr *SimpleRedis) logDebugCanceled(ctx context.Context) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgCanceled, "host", sr.host)
	}
}

// logDebugSocketClosed is simpleredis_socket_closed for a benign destroy.
func (sr *SimpleRedis) logDebugSocketClosed(ctx context.Context, reason string) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgSocketClosed, "reason", reason)
	}
}

// logDebugDial is simpleredis_dial after TCP succeeds.
func (sr *SimpleRedis) logDebugDial(ctx context.Context, reason string) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgDial, "reason", reason, "host", sr.host)
	}
}

// logDebugIdleSwept is simpleredis_idle_swept after borrow closes stale sockets.
func (sr *SimpleRedis) logDebugIdleSwept(ctx context.Context, swept, remaining int) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgIdleSwept, "swept", swept, "remaining", remaining)
	}
}

// logDebugRetry is simpleredis_retry before the backoff sleep.
func (sr *SimpleRedis) logDebugRetry(ctx context.Context, attempt int, backoff time.Duration, retryErr error) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgRetry, "attempt", attempt, "backoff", backoff, "error", retryErr)
	}
}

// logDebugCapability is simpleredis_capability when MSetEX caches native vs lua.
func (sr *SimpleRedis) logDebugCapability(ctx context.Context, path string) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgCapability, "path", path)
	}
}

// logDebugNoScript is simpleredis_noscript when EVALSHA misses and EVAL is sent.
func (sr *SimpleRedis) logDebugNoScript(ctx context.Context) {
	if logger, ok := sr.debugLogger(ctx); ok {
		logger.Debug(MsgNoScript, "host", sr.host)
	}
}

// logDebugOpen is simpleredis_open at New. Never logs Pass.
func (sr *SimpleRedis) logDebugOpen() {
	if logger, ok := sr.debugLogger(context.Background()); ok {
		logger.Debug(MsgOpen,
			"host", sr.host,
			"database", sr.database,
			"pool_size", sr.poolSize,
			"max_idle_conns", sr.maxIdleConns,
			"pool_timeout", sr.poolTimeout,
			"idle_timeout", sr.idleTimeout,
			"dial_timeout", sr.dialTimeout,
			"io_timeout", sr.ioTimeout,
			"max_retries", sr.maxRetries,
			"min_retry_backoff", sr.minRetryBackoff,
			"max_retry_backoff", sr.maxRetryBackoff,
		)
	}
}

// logDebugClose is simpleredis_close after idle sockets are drained.
func (sr *SimpleRedis) logDebugClose(idleClosed int) {
	if logger, ok := sr.debugLogger(context.Background()); ok {
		logger.Debug(MsgClose, "idle_closed", idleClosed)
	}
}

// commandError maps library deadline to redis:timeout and logs that conversion.
// I/O timeouts and mid-command cancel are logged at the site that detected them.
func (sr *SimpleRedis) commandError(ctx context.Context, err error, libraryOwnsDeadline bool) error {
	if err == errTimeout { //nolint:errorlint // already mapped and logged at the I/O site
		return err
	}
	mapped := libraryTimeout(err, libraryOwnsDeadline)
	if mapped == errTimeout { //nolint:errorlint // exact mapped sentinel
		sr.logDebugTimeout(ctx, mapped)
	}
	return mapped
}

// errorsIsCanceled is context.Canceled, including wrapping.
func errorsIsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
