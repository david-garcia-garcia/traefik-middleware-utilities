// Package simpleredisprobe is a Traefik local plugin that Inits SimpleRedis
// and runs every client verb on each request so Pester can prove the library loads under Yaegi.
package simpleredisprobe

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const defaultHost = "redis:6379"

// kongIncrbyExpireatScript is the Kong flush snippet (KEYS declared, Lua 5.1-safe).
const kongIncrbyExpireatScript = `local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value`

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

// ServeHTTP runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval, and a missing-key Get, then copies results into headers.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	prefix := fmt.Sprintf("srp:%d", time.Now().UnixNano())
	setKey := prefix + ":set"
	incrKey := prefix + ":incr"
	incrByKey := prefix + ":incrby"
	expireKey := prefix + ":expire"
	expireAtKey := prefix + ":expireat"
	evalKey := prefix + ":eval"
	delKey := prefix + ":del"

	if err := m.client.Set(setKey, []byte("ok"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	getValue, err := m.client.Get(setKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Value", string(getValue))

	slots, err := m.client.MGet([]string{setKey, prefix + ":missing"})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(slots) != 2 || string(slots[0]) != "ok" || slots[1] != nil {
		http.Error(rw, "mget slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MGet", string(slots[0]))

	if err := m.client.Set(delKey, []byte("gone"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Del(delKey); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Del", "ok")

	incrValue, err := m.client.Incr(incrKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Incr", strconv.FormatInt(incrValue, 10))

	incrByValue, err := m.client.IncrBy(incrByKey, 5)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-IncrBy", strconv.FormatInt(incrByValue, 10))

	if err := m.client.Set(expireKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Expire(expireKey, 30); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Expire", "ok")

	if err := m.client.Set(expireAtKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.ExpireAt(expireAtKey, time.Now().Add(60*time.Second).Unix()); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-ExpireAt", "ok")

	evalValues, err := m.client.Eval(kongIncrbyExpireatScript, []string{evalKey}, []string{"3", strconv.FormatInt(time.Now().Add(60*time.Second).Unix(), 10)})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(evalValues) != 1 {
		http.Error(rw, "eval slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Eval", string(evalValues[0]))

	_, missErr := m.client.Get(prefix + ":missing")
	if missErr == nil || missErr.Error() != simpleredis.RedisMiss {
		http.Error(rw, "get miss", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-GetMiss", simpleredis.RedisMiss)

	m.next.ServeHTTP(rw, req)
}
