package simpleredis

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultMaxIdleConns = 8
	defaultPoolSize     = 8
	defaultIdleTimeout  = 30 * time.Second
	defaultDialTimeout  = 200 * time.Millisecond
	defaultIOTimeout    = 250 * time.Millisecond
	defaultPoolTimeout  = 200 * time.Millisecond
	defaultMaxRetries   = 1
)

// ErrMaxIdleConnsAbovePoolSize is New when an explicit MaxIdleConns is above PoolSize.
// The idle list cannot hold more sockets than the live cap, so that Config cannot build a client.
var ErrMaxIdleConnsAbovePoolSize = errors.New("simpleredis: MaxIdleConns must not exceed PoolSize")

// Config is the freeze-at-New settings for a SimpleRedis client.
// New copies these values onto the client. Later writes to this struct do not change a client that already ran New.
// A zero Config uses the package defaults (live cap 8, idle trim 8, 200ms pool wait, 30s idle reuse gate, 200ms dial, 250ms I/O, 1 extra retry).
// An explicit MaxIdleConns above PoolSize returns ErrMaxIdleConnsAbovePoolSize and no client. A zero MaxIdleConns is not a request for 8: it follows a smaller PoolSize, so PoolSize alone still builds. A trim below PoolSize stays as written: that trades a smaller idle footprint for closing (and later re-dialling) every socket released above the trim.
// Worst-case command wait is (MaxRetries+1)*(DialTimeout+IOTimeout) (900ms at those defaults). Min/max backoff keep go-redis sentinels: 0 means 8ms / 512ms; -1 means off. MaxRetries 0 at New means 1 extra retry; -1 means none.
type Config struct {
	// Host is the TCP address New stores (host:port). New does not dial.
	Host string
	// Pass is AUTH on each new dial when non-empty.
	Pass string
	// Database is SELECT on each new dial when non-empty.
	Database string

	// PoolSize is the live-socket cap (idle plus in-use). 0 means 8.
	PoolSize int
	// MaxIdleConns is how many unused sockets release will keep. 0 means 8, or PoolSize when that is smaller.
	// New rejects the Config when the value after that default is above PoolSize.
	MaxIdleConns int
	// PoolTimeout is how long a waiter past PoolSize blocks. 0 means 200ms.
	PoolTimeout time.Duration
	// IdleTimeout is how long an idle socket may sit before borrow refuses to reuse it. 0 means 30s.
	IdleTimeout time.Duration
	// DialTimeout is the TCP dial bound. 0 means 200ms.
	DialTimeout time.Duration
	// IOTimeout is this hop's I/O share of the command budget, not a per-operation cap. 0 means 250ms.
	// The socket deadline is what is left of (MaxRetries+1)*(DialTimeout+IOTimeout), so a large reply
	// that keeps arriving is not cut off for its size while a quiet peer still ends the command.
	IOTimeout time.Duration

	// MaxRetries is extra retries after the first attempt. 0 at New means 1; -1 means none (one send).
	MaxRetries int
	// MinRetryBackoff is the base backoff between retries. 0 means 8ms; -1 means no sleep.
	MinRetryBackoff time.Duration
	// MaxRetryBackoff caps backoff. 0 means 512ms; -1 means 0.
	MaxRetryBackoff time.Duration
}

// applyDefaults fills zero pool, timeout, and MaxRetries knobs, clamps a defaulted MaxIdleConns to PoolSize, and rejects an explicit MaxIdleConns above PoolSize. Min/max backoff sentinels stay 0/-1 for retryLimits.
func (cfg Config) applyDefaults() (Config, error) {
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = defaultPoolSize
	}
	if cfg.MaxIdleConns <= 0 {
		// Unset is not a request for 8: follow a smaller PoolSize instead of rejecting the Config.
		cfg.MaxIdleConns = defaultMaxIdleConns
		if cfg.MaxIdleConns > cfg.PoolSize {
			cfg.MaxIdleConns = cfg.PoolSize
		}
	} else if cfg.MaxIdleConns > cfg.PoolSize {
		return Config{}, fmt.Errorf("%w (MaxIdleConns=%d PoolSize=%d)", ErrMaxIdleConnsAbovePoolSize, cfg.MaxIdleConns, cfg.PoolSize)
	}
	if cfg.PoolTimeout <= 0 {
		cfg.PoolTimeout = defaultPoolTimeout
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultDialTimeout
	}
	if cfg.IOTimeout <= 0 {
		cfg.IOTimeout = defaultIOTimeout
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	return cfg, nil
}
