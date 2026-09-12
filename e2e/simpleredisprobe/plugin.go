// Package simpleredisprobe is a Traefik local plugin that builds SimpleRedis
// with simpleredis.New and runs every client verb on each request so Pester can prove the library loads under Yaegi.
package simpleredisprobe

import (
	"context"
	"crypto/sha1" //nolint:gosec // Redis EVALSHA digest is SHA-1
	"encoding/hex"
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

// timeWaitHoldScript busy-waits ARGV microseconds via TIME so ?hold= occupies a poolSize turn. Zero KEYS; Lua 5.1-safe (no table.maxn).
const timeWaitHoldScript = `local start = redis.call("TIME")
local startSec = tonumber(start[1])
local startUsec = tonumber(start[2])
local need = tonumber(ARGV[1])
while true do
  local now = redis.call("TIME")
  local elapsed = (tonumber(now[1]) - startSec) * 1000000 + (tonumber(now[2]) - startUsec)
  if elapsed >= need then
    break
  end
end
return 1`

// ttlScript returns TTL for KEYS[1] so Pester can assert MSetEX expiry landed.
const ttlScript = `return redis.call('TTL', KEYS[1])`

// kongScriptDigest is SHA-1 hex of kongIncrbyExpireatScript (Redis sha1hex).
func kongScriptDigest() string {
	sum := sha1.Sum([]byte(kongIncrbyExpireatScript)) //nolint:gosec // Redis EVALSHA digest is SHA-1
	return hex.EncodeToString(sum[:])
}

// Config is the dynamic plugin settings Traefik decodes.
type Config struct {
	Host     string `json:"host,omitempty" yaml:"host,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	Database string `json:"database,omitempty" yaml:"database,omitempty"`
	DropHost string `json:"dropHost,omitempty" yaml:"dropHost,omitempty"`
}

// CreateConfig returns default plugin settings (compose Redis, no password, empty database).
func CreateConfig() *Config {
	return &Config{Host: defaultHost}
}

// middleware holds the SimpleRedis clients and the next handler in the Traefik chain.
type middleware struct {
	next       http.Handler
	client     *simpleredis.SimpleRedis
	dropClient *simpleredis.SimpleRedis
}

// New constructs SimpleRedis from Config and returns a handler. It does not dial.
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

	// ?hold=500000 busy-waits 500ms; zero-Config IOTimeout is 100ms and would abort the hold.
	client := simpleredis.New(simpleredis.Config{Host: host, Pass: cfg.Password, Database: cfg.Database, IOTimeout: time.Second})
	mw := &middleware{next: next, client: client}
	if cfg.DropHost != "" {
		mw.dropClient = simpleredis.New(simpleredis.Config{Host: cfg.DropHost})
	}
	return mw, nil
}

// ServeHTTP runs Set+Get only when recover=1; otherwise optionally holds via ?hold=, then runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval twice, a missing-key Get, MSetEX, and MSetEXAt, and copies results into headers. When dropClient is set, it also warms that client and sets DropIncr/DropEval headers.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if req.URL.Query().Get("recover") == "1" {
		m.serveRecover(rw, req)
		return
	}

	// Hold occupies a live turn so Pester can contend for poolSize.
	if hold := req.URL.Query().Get("hold"); hold != "" {
		if _, err := m.client.Eval(ctx, timeWaitHoldScript, nil, []string{hold}); err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}
		rw.Header().Set("X-SimpleRedis-Hold", "ok")
	}

	prefix := fmt.Sprintf("srp:%d", time.Now().UnixNano())
	setKey := prefix + ":set"
	incrKey := prefix + ":incr"
	incrByKey := prefix + ":incrby"
	expireKey := prefix + ":expire"
	expireAtKey := prefix + ":expireat"
	evalKey := prefix + ":eval"
	evalAgainKey := prefix + ":eval2"
	delKey := prefix + ":del"

	if err := m.client.Set(ctx, setKey, []byte(prefix), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	getValue, err := m.client.Get(ctx, setKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Value", string(getValue))

	slots, err := m.client.MGet(ctx, []string{setKey, prefix + ":missing"})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(slots) != 2 || string(slots[0]) != prefix || slots[1] != nil {
		http.Error(rw, "mget slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MGet", string(slots[0]))

	if err := m.client.Set(ctx, delKey, []byte("gone"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Del(ctx, delKey); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Del", "ok")

	incrValue, err := m.client.Incr(ctx, incrKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Incr", strconv.FormatInt(incrValue, 10))

	incrByValue, err := m.client.IncrBy(ctx, incrByKey, 5)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-IncrBy", strconv.FormatInt(incrByValue, 10))

	if err := m.client.Set(ctx, expireKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Expire(ctx, expireKey, 30); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Expire", "ok")

	if err := m.client.Set(ctx, expireAtKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.ExpireAt(ctx, expireAtKey, time.Now().Add(60*time.Second).Unix()); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-ExpireAt", "ok")

	expireUnix := strconv.FormatInt(time.Now().Add(60*time.Second).Unix(), 10)
	evalValues, err := m.client.Eval(ctx, kongIncrbyExpireatScript, []string{evalKey}, []string{"3", expireUnix})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(evalValues) != 1 {
		http.Error(rw, "eval slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Eval", string(evalValues[0]))

	evalAgainValues, err := m.client.Eval(ctx, kongIncrbyExpireatScript, []string{evalAgainKey}, []string{"3", expireUnix})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(evalAgainValues) != 1 {
		http.Error(rw, "eval again slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-EvalAgain", string(evalAgainValues[0]))
	rw.Header().Set("X-SimpleRedis-EvalDigest", kongScriptDigest())

	_, missErr := m.client.Get(ctx, prefix+":missing")
	if missErr == nil || missErr.Error() != simpleredis.RedisMiss {
		http.Error(rw, "get miss", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-GetMiss", simpleredis.RedisMiss)

	msetexKey := prefix + ":msetex"
	msetexAtKey := prefix + ":msetexat"
	if err := m.client.MSetEX(ctx, []string{msetexKey}, [][]byte{[]byte("ok")}, 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MSetEX", "ok")
	ttlValues, err := m.client.Eval(ctx, ttlScript, []string{msetexKey}, nil)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(ttlValues) != 1 {
		http.Error(rw, "msetex ttl slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MSetEX-TTL", string(ttlValues[0]))

	if err := m.client.MSetEXAt(ctx, []string{msetexAtKey}, [][]byte{[]byte("ok")}, time.Now().Add(60*time.Second).Unix()); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}

	if m.dropClient != nil {
		m.writeDropHeaders(ctx, rw, prefix)
	}

	m.next.ServeHTTP(rw, req)
}

// serveRecover runs Set+Get only and sets X-SimpleRedis-Recover when those succeed. It does not Eval.
func (m *middleware) serveRecover(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	prefix := fmt.Sprintf("srp:%d", time.Now().UnixNano())
	setKey := prefix + ":set"
	if err := m.client.Set(ctx, setKey, []byte("ok"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if _, err := m.client.Get(ctx, setKey); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Recover", "ok")
	m.next.ServeHTTP(rw, req)
}

// writeDropHeaders warms the drop-relay pool, then Incr and Eval through it (lost reply is retried; stored values are double-apply), and reads stored values from the engine client.
func (m *middleware) writeDropHeaders(ctx context.Context, rw http.ResponseWriter, prefix string) {
	dropIncrKey := prefix + ":dropincr"
	dropEvalKey := prefix + ":dropeval"

	// Warm so Incr is a reused socket; GET miss still pools. Lost-reply Incr then retries on a new session whose first command is INCR and must succeed (double-apply).
	_, _ = m.dropClient.Get(ctx, prefix+":dropwarm")
	incrValue, incrErr := m.dropClient.Incr(ctx, dropIncrKey)
	rw.Header().Set("X-SimpleRedis-DropIncr", dropResultText(incrErr, strconv.FormatInt(incrValue, 10)))
	incrStored, incrStoredErr := m.client.Get(ctx, dropIncrKey)
	rw.Header().Set("X-SimpleRedis-DropIncrStored", storedText(incrStored, incrStoredErr))

	// Warm again so Eval is also a reused socket (Incr closed the previous one).
	_, _ = m.dropClient.Get(ctx, prefix+":dropwarm2")
	evalValues, evalErr := m.dropClient.Eval(ctx, kongIncrbyExpireatScript, []string{dropEvalKey}, []string{"3", strconv.FormatInt(time.Now().Add(60*time.Second).Unix(), 10)})
	evalText := "ok"
	if len(evalValues) == 1 {
		evalText = string(evalValues[0])
	}
	rw.Header().Set("X-SimpleRedis-DropEval", dropResultText(evalErr, evalText))
	evalStored, evalStoredErr := m.client.Get(ctx, dropEvalKey)
	rw.Header().Set("X-SimpleRedis-DropEvalStored", storedText(evalStored, evalStoredErr))
}

// dropResultText is the integer reply when the drop command succeeded, or err.Error when it failed.
func dropResultText(err error, success string) string {
	if err != nil {
		return err.Error()
	}
	return success
}

// storedText is the GET payload, or the GET error text when the engine key is missing.
func storedText(value []byte, err error) string {
	if err != nil {
		return err.Error()
	}
	return string(value)
}
