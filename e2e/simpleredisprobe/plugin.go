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

// kongScriptDigest is SHA-1 hex of kongIncrbyExpireatScript (Redis sha1hex).
func kongScriptDigest() string {
	sum := sha1.Sum([]byte(kongIncrbyExpireatScript)) //nolint:gosec // Redis EVALSHA digest is SHA-1
	return hex.EncodeToString(sum[:])
}

// Config is the dynamic plugin settings Traefik decodes.
type Config struct {
	Host     string `json:"host,omitempty" yaml:"host,omitempty"`
	DropHost string `json:"dropHost,omitempty" yaml:"dropHost,omitempty"`
}

// CreateConfig returns default plugin settings (compose Redis, no password).
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

	client := simpleredis.New(simpleredis.Config{Host: host})
	mw := &middleware{next: next, client: client}
	if cfg.DropHost != "" {
		mw.dropClient = simpleredis.New(simpleredis.Config{Host: cfg.DropHost})
	}
	return mw, nil
}

// ServeHTTP optionally holds one pool socket via ?hold= microseconds (Eval TIME-wait) and returns. Without hold it runs Set, Get, MGet, Del, Incr, IncrBy, Expire, ExpireAt, Eval twice, and one mixed ExecPipeline, and copies results into headers. When dropClient is set, it also warms that client and sets DropIncr/DropEval headers.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Hold occupies a live turn so Pester can contend for poolSize. Return after the wait; verb and pipeline headers are the non-hold path.
	if hold := req.URL.Query().Get("hold"); hold != "" {
		if _, err := m.client.Eval(timeWaitHoldScript, nil, []string{hold}); err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}
		rw.Header().Set("X-SimpleRedis-Hold", "ok")
		m.next.ServeHTTP(rw, req)
		return
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
	pipeIncrKey := prefix + ":pipeincr"
	pipeEvalKey := prefix + ":pipeeval"

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

	evalAgainValues, err := m.client.Eval(kongIncrbyExpireatScript, []string{evalAgainKey}, []string{"3", expireUnix})
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

	// Mixed INCR+EXPIRE+GET+EVAL on unique keys; EVAL lists KEYS[1] (Lua 5.1-safe).
	pipeSlots, err := m.client.ExecPipeline([][][]byte{
		{[]byte("INCR"), []byte(pipeIncrKey)},
		{[]byte("EXPIRE"), []byte(pipeIncrKey), []byte("60")},
		{[]byte("GET"), []byte(pipeIncrKey)},
		{[]byte("EVAL"), []byte(kongIncrbyExpireatScript), []byte("1"), []byte(pipeEvalKey), []byte("3"), []byte(expireUnix)},
	})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	if len(pipeSlots) != 4 {
		http.Error(rw, "pipeline slots", http.StatusBadGateway)
		return
	}
	if pipeSlots[0].Err != nil || len(pipeSlots[0].Values) != 1 {
		http.Error(rw, "pipeline incr", http.StatusBadGateway)
		return
	}
	if pipeSlots[1].Err != nil {
		http.Error(rw, "pipeline expire", http.StatusBadGateway)
		return
	}
	if pipeSlots[2].Err != nil || len(pipeSlots[2].Values) != 1 {
		http.Error(rw, "pipeline get", http.StatusBadGateway)
		return
	}
	if pipeSlots[3].Err != nil || len(pipeSlots[3].Values) != 1 {
		http.Error(rw, "pipeline eval", http.StatusBadGateway)
		return
	}
	rw.Header().Set("X-SimpleRedis-Pipeline", string(pipeSlots[0].Values[0])+":ok:"+string(pipeSlots[2].Values[0])+":"+string(pipeSlots[3].Values[0]))

	if m.dropClient != nil {
		m.writeDropHeaders(rw, prefix)
	}

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
