// Package simpleredisprobe is a Traefik local plugin that Inits SimpleRedis
// and SET+GETs on each request so Pester can prove the library loads under Yaegi.
package simpleredisprobe

import (
	"context"
	"fmt"
	"net/http"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const defaultHost = "redis:6379"

// Config is the dynamic plugin settings Traefik decodes.
type Config struct {
	Host string `json:"host,omitempty" yaml:"host,omitempty"`
}

// CreateConfig returns default plugin settings (compose Redis, no password).
func CreateConfig() *Config {
	return &Config{Host: defaultHost}
}

// middleware holds the Inited client and the next handler in the Traefik chain.
type middleware struct {
	next   http.Handler
	client *simpleredis.SimpleRedis
}

// New Inits SimpleRedis from Config and returns a handler. It does not dial.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	_ = ctx
	_ = name
	if next == nil {
		return nil, fmt.Errorf("simpleredisprobe: missing next handler")
	}
	if cfg == nil {
		cfg = CreateConfig()
	}
	host := cfg.Host
	if host == "" {
		host = defaultHost
	}

	client := &simpleredis.SimpleRedis{}
	client.Init(host, "", "")
	return &middleware{next: next, client: client}, nil
}

// ServeHTTP SETs a probe key, GETs it back, and copies those bytes into X-SimpleRedis-Value.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	key := "simpleredisprobe"
	value := []byte("ok")
	if err := m.client.Set(key, value, 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	got, err := m.client.Get(key)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Value", string(got))
	m.next.ServeHTTP(rw, req)
}
