package reclaim

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

var (
	defaultMu    sync.Mutex
	defaultTable *Table
)

// Default returns the process-wide table, creating it on first use.
func Default() *Table {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable == nil {
		defaultTable = NewTable(DefaultGrace)
	}
	return defaultTable
}

// Open is Default().Open: create-once for key on the process table and bind ctx.
// logger is required. hooks are stored at put; a later Open ignores them. Nil hook funcs skip.
func Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error) {
	return Default().Open(ctx, key, logger, create, hooks)
}

// Reset tears down the process table (sleeps then closes every incarnation) and installs a fresh
// one. Tests only.
func Reset() {
	ResetWith(DefaultGrace)
}

// ResetWith replaces the process table after ending every incarnation on the current one, with
// grace as how long a sleeping value is kept. Tests only.
func ResetWith(grace time.Duration) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable != nil {
		defaultTable.Reset()
	}
	defaultTable = NewTable(grace)
}
