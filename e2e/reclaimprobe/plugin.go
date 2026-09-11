// Package reclaimprobe is a Traefik local plugin that opens one reclaim key
// so Pester can prove the library loads under Yaegi.
package reclaimprobe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

// Config is the dynamic plugin settings Traefik decodes.
type Config struct {
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
}

// CreateConfig returns default plugin settings.
func CreateConfig() *Config {
	return &Config{Key: "probe"}
}

// probeValue is the stored reclaim value. Create assigns a monotonic id.
type probeValue struct {
	id int64
}

var nextID atomic.Int64

// middleware is the HTTP handler Traefik chains in front of whoami.
type middleware struct {
	next http.Handler
	id   string
	name string
}

// New opens the configured reclaim key and returns a passthrough handler.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("reclaimprobe: missing next handler")
	}
	if cfg == nil {
		cfg = CreateConfig()
	}
	key := cfg.Key
	if key == "" {
		key = "probe"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	stored, err := reclaim.Open(ctx, key, logger, func() (any, error) {
		return &probeValue{id: nextID.Add(1)}, nil
	})
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%T", stored)
	if typed, ok := stored.(*probeValue); ok {
		id = fmt.Sprintf("%d", typed.id)
	}

	return &middleware{next: next, id: id, name: name}, nil
}

// ServeHTTP adds reclaim identity headers and calls the next handler.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("X-Reclaim-ID", m.id)
	rw.Header().Set("X-Reclaim-Name", m.name)
	m.next.ServeHTTP(rw, req)
}
