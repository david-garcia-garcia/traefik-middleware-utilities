// Package simpleredisprobe is a Traefik local plugin that builds SimpleRedis
// with simpleredis.New and runs one client verb per path case so Pester can prove the library loads under Yaegi.
package simpleredisprobe

import (
	"context"
	"crypto/sha1" //nolint:gosec // Redis EVALSHA digest is SHA-1
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// timeWaitHoldScript busy-waits ARGV microseconds via TIME so /hold occupies a poolSize turn. Zero KEYS; Lua 5.1-safe (no table.maxn).
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

// requestPathCase is health when the path has one segment, the second segment when there are two, or empty when the remainder is unknown.
func requestPathCase(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return "health"
	}
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// requestKeyPrefix is a unique key prefix for one probe request.
func requestKeyPrefix() string {
	return fmt.Sprintf("srp:%d", time.Now().UnixNano())
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

	client := simpleredis.New(simpleredis.Config{Host: host, Pass: cfg.Password, Database: cfg.Database})
	mw := &middleware{next: next, client: client}
	if cfg.DropHost != "" {
		mw.dropClient = simpleredis.New(simpleredis.Config{Host: cfg.DropHost})
	}
	return mw, nil
}

// ServeHTTP runs the path case (health Set+Get, one verb, recover, drop, or hold) and copies that case's result into headers.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	pathCase := requestPathCase(req.URL.Path)
	if pathCase == "" {
		http.NotFound(rw, req)
		return
	}
	switch pathCase {
	case "health", "get":
		m.serveGet(rw, req)
	case "mget":
		m.serveMGet(rw, req)
	case "del":
		m.serveDel(rw, req)
	case "incr":
		m.serveIncr(rw, req)
	case "incrby":
		m.serveIncrBy(rw, req)
	case "expire":
		m.serveExpire(rw, req)
	case "expireat":
		m.serveExpireAt(rw, req)
	case "eval":
		m.serveEval(rw, req)
	case "get-miss":
		m.serveGetMiss(rw, req)
	case "msetex":
		m.serveMSetEX(rw, req)
	case "msetexat":
		m.serveMSetEXAt(rw, req)
	case "recover":
		m.serveRecover(rw, req)
	case "drop":
		m.serveDrop(rw, req)
	case "hold":
		m.serveHold(rw, req)
	default:
		http.NotFound(rw, req)
	}
}

// serveGet Sets a unique token then Gets it into X-SimpleRedis-Value.
func (m *middleware) serveGet(rw http.ResponseWriter, req *http.Request) {
	prefix := requestKeyPrefix()
	setKey := prefix + ":set"
	if err := m.client.Set(setKey, []byte(prefix), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	getValue, err := m.client.Get(setKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Value", string(getValue))
	m.next.ServeHTTP(rw, req)
}

// serveMGet Sets a key then MGets that key and a missing name into X-SimpleRedis-MGet.
func (m *middleware) serveMGet(rw http.ResponseWriter, req *http.Request) {
	prefix := requestKeyPrefix()
	setKey := prefix + ":set"
	if err := m.client.Set(setKey, []byte(prefix), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	slots, err := m.client.MGet([]string{setKey, prefix + ":missing"})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(slots) != 2 || string(slots[0]) != prefix || slots[1] != nil {
		http.Error(rw, "mget slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MGet", string(slots[0]))
	m.next.ServeHTTP(rw, req)
}

// serveDel Sets a key then Dels it.
func (m *middleware) serveDel(rw http.ResponseWriter, req *http.Request) {
	prefix := requestKeyPrefix()
	delKey := prefix + ":del"
	if err := m.client.Set(delKey, []byte("gone"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Del(delKey); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Del", "ok")
	m.next.ServeHTTP(rw, req)
}

// serveIncr Incrs a missing key into X-SimpleRedis-Incr.
func (m *middleware) serveIncr(rw http.ResponseWriter, req *http.Request) {
	incrValue, err := m.client.Incr(requestKeyPrefix() + ":incr")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Incr", strconv.FormatInt(incrValue, 10))
	m.next.ServeHTTP(rw, req)
}

// serveIncrBy IncrBys a missing key by 5 into X-SimpleRedis-IncrBy.
func (m *middleware) serveIncrBy(rw http.ResponseWriter, req *http.Request) {
	incrByValue, err := m.client.IncrBy(requestKeyPrefix()+":incrby", 5)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-IncrBy", strconv.FormatInt(incrByValue, 10))
	m.next.ServeHTTP(rw, req)
}

// serveExpire Sets a key then Expires it.
func (m *middleware) serveExpire(rw http.ResponseWriter, req *http.Request) {
	expireKey := requestKeyPrefix() + ":expire"
	if err := m.client.Set(expireKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.Expire(expireKey, 30); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Expire", "ok")
	m.next.ServeHTTP(rw, req)
}

// serveExpireAt Sets a key then ExpireAts it.
func (m *middleware) serveExpireAt(rw http.ResponseWriter, req *http.Request) {
	expireAtKey := requestKeyPrefix() + ":expireat"
	if err := m.client.Set(expireAtKey, []byte("1"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if err := m.client.ExpireAt(expireAtKey, time.Now().Add(60*time.Second).Unix()); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-ExpireAt", "ok")
	m.next.ServeHTTP(rw, req)
}

// serveEval Evals the Kong KEYS snippet once and sets Eval plus digest headers.
func (m *middleware) serveEval(rw http.ResponseWriter, req *http.Request) {
	evalKey := requestKeyPrefix() + ":eval"
	expireUnix := strconv.FormatInt(time.Now().Add(60*time.Second).Unix(), 10)
	evalValues, err := m.client.Eval(kongIncrbyExpireatScript, []string{evalKey}, []string{"3", expireUnix})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(evalValues) != 1 {
		http.Error(rw, "eval slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Eval", string(evalValues[0]))
	rw.Header().Set("X-SimpleRedis-EvalDigest", kongScriptDigest())
	m.next.ServeHTTP(rw, req)
}

// serveGetMiss Gets a missing key and sets X-SimpleRedis-GetMiss to redis:miss.
func (m *middleware) serveGetMiss(rw http.ResponseWriter, req *http.Request) {
	_, missErr := m.client.Get(requestKeyPrefix() + ":missing")
	if missErr == nil || missErr.Error() != simpleredis.RedisMiss {
		http.Error(rw, "get miss", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-GetMiss", simpleredis.RedisMiss)
	m.next.ServeHTTP(rw, req)
}

// serveMSetEX writes one pair then Evals TTL into X-SimpleRedis-MSetEX and X-SimpleRedis-MSetEX-TTL.
func (m *middleware) serveMSetEX(rw http.ResponseWriter, req *http.Request) {
	msetexKey := requestKeyPrefix() + ":msetex"
	if err := m.client.MSetEX([]string{msetexKey}, [][]byte{[]byte("ok")}, 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MSetEX", "ok")
	ttlValues, err := m.client.Eval(ttlScript, []string{msetexKey}, nil)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(ttlValues) != 1 {
		http.Error(rw, "msetex ttl slots", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MSetEX-TTL", string(ttlValues[0]))
	m.next.ServeHTTP(rw, req)
}

// serveMSetEXAt writes one pair with a future Unix time then Gets it.
func (m *middleware) serveMSetEXAt(rw http.ResponseWriter, req *http.Request) {
	msetexAtKey := requestKeyPrefix() + ":msetexat"
	if err := m.client.MSetEXAt([]string{msetexAtKey}, [][]byte{[]byte("ok")}, time.Now().Add(60*time.Second).Unix()); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	got, err := m.client.Get(msetexAtKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-MSetEXAt", string(got))
	m.next.ServeHTTP(rw, req)
}

// serveRecover runs Set+Get only and sets X-SimpleRedis-Recover when those succeed. It does not Eval.
func (m *middleware) serveRecover(rw http.ResponseWriter, req *http.Request) {
	prefix := requestKeyPrefix()
	setKey := prefix + ":set"
	if err := m.client.Set(setKey, []byte("ok"), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if _, err := m.client.Get(setKey); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Recover", "ok")
	m.next.ServeHTTP(rw, req)
}

// serveDrop warms the drop-relay pool then Incr and Eval through it.
func (m *middleware) serveDrop(rw http.ResponseWriter, req *http.Request) {
	if m.dropClient == nil {
		http.Error(rw, "dropHost unset", http.StatusBadGateway)
		return
	}
	m.writeDropHeaders(rw, requestKeyPrefix())
	m.next.ServeHTTP(rw, req)
}

// serveHold occupies a live turn for ?hold= microseconds so Pester can contend for poolSize.
func (m *middleware) serveHold(rw http.ResponseWriter, req *http.Request) {
	hold := req.URL.Query().Get("hold")
	if hold == "" {
		http.Error(rw, "hold query required", http.StatusBadRequest)
		return
	}
	if _, err := m.client.Eval(timeWaitHoldScript, nil, []string{hold}); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Hold", "ok")
	m.next.ServeHTTP(rw, req)
}

// writeDropHeaders warms the drop-relay pool, then Incr and Eval through it (lost reply is retried; stored values are double-apply), and reads stored values from the engine client.
func (m *middleware) writeDropHeaders(rw http.ResponseWriter, prefix string) {
	dropIncrKey := prefix + ":dropincr"
	dropEvalKey := prefix + ":dropeval"

	// Warm so Incr is a reused socket; GET miss still pools. Lost-reply Incr then retries on a new session whose first command is INCR and must succeed (double-apply).
	_, _ = m.dropClient.Get(prefix + ":dropwarm")
	incrValue, incrErr := m.dropClient.Incr(dropIncrKey)
	rw.Header().Set("X-SimpleRedis-DropIncr", dropResultText(incrErr, strconv.FormatInt(incrValue, 10)))
	incrStored, incrStoredErr := m.client.Get(dropIncrKey)
	rw.Header().Set("X-SimpleRedis-DropIncrStored", storedText(incrStored, incrStoredErr))

	// Warm again so Eval is also a reused socket (Incr closed the previous one).
	_, _ = m.dropClient.Get(prefix + ":dropwarm2")
	evalValues, evalErr := m.dropClient.Eval(kongIncrbyExpireatScript, []string{dropEvalKey}, []string{"3", strconv.FormatInt(time.Now().Add(60*time.Second).Unix(), 10)})
	evalText := "ok"
	if len(evalValues) == 1 {
		evalText = string(evalValues[0])
	}
	rw.Header().Set("X-SimpleRedis-DropEval", dropResultText(evalErr, evalText))
	evalStored, evalStoredErr := m.client.Get(dropEvalKey)
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
