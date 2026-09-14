// Package simpleredisprobe is a Traefik local plugin that builds SimpleRedis
// with simpleredis.New and maps each public verb to an HTTP path so Pester can
// drive the client from the request.
package simpleredisprobe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

const defaultHost = "redis:6379"

const maxBodyBytes = 1 << 20

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

// requestKeyPrefix is a unique key prefix for one health request.
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

// middleware holds the SimpleRedis clients built in New.
type middleware struct {
	client     *simpleredis.SimpleRedis
	dropClient *simpleredis.SimpleRedis
}

// New constructs SimpleRedis from Config and returns a terminal handler. It does not dial.
// Traefik still passes next; ServeHTTP never forwards it.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	_ = ctx
	_ = name
	if next == nil {
		return nil, fmt.Errorf("simpleredisprobe: Traefik next handler is required (probe responses are terminal)")
	}
	if cfg == nil {
		cfg = CreateConfig()
	}
	host := cfg.Host
	if host == "" {
		host = defaultHost
	}

	// Pester may Eval a 500ms TIME wait; the zero-Config 900ms command budget would abort that command.
	client, err := simpleredis.New(simpleredis.Config{Host: host, Pass: cfg.Password, Database: cfg.Database, CommandTimeout: 3 * time.Second})
	if err != nil {
		return nil, err
	}
	mw := &middleware{client: client}
	if cfg.DropHost != "" {
		dropClient, err := simpleredis.New(simpleredis.Config{Host: cfg.DropHost, CommandTimeout: 3 * time.Second})
		if err != nil {
			return nil, err
		}
		mw.dropClient = dropClient
	}
	return mw, nil
}

// ServeHTTP maps the last path segment to one SimpleRedis verb, or health Set+Get on a one-segment path.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	pathCase := requestPathCase(req.URL.Path)
	if pathCase == "" {
		http.NotFound(rw, req)
		return
	}
	if pathCase == "health" {
		m.serveHealth(rw, req)
		return
	}
	client, err := m.commandClient(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	ctx := req.Context()
	q := req.URL.Query()
	keys := q["key"]
	args := q["arg"]
	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes+1))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body) > maxBodyBytes {
		http.Error(rw, "body too large", http.StatusBadRequest)
		return
	}
	switch pathCase {
	case "get":
		m.serveGet(rw, ctx, client, keys)
	case "mget":
		m.serveMGet(rw, ctx, client, keys)
	case "set":
		m.serveSet(rw, ctx, client, keys, q.Get("ex"), body)
	case "del":
		m.serveDel(rw, ctx, client, keys)
	case "incr":
		m.serveIncr(rw, ctx, client, keys)
	case "incrby":
		m.serveIncrBy(rw, ctx, client, keys, q.Get("delta"))
	case "expire":
		m.serveExpire(rw, ctx, client, keys, q.Get("ex"))
	case "expireat":
		m.serveExpireAt(rw, ctx, client, keys, q.Get("at"))
	case "eval":
		m.serveEval(rw, ctx, client, keys, args, q.Get("digest"), body)
	case "msetex":
		m.serveMSetEX(rw, ctx, client, keys, q.Get("ex"), body)
	case "msetexat":
		m.serveMSetEXAt(rw, ctx, client, keys, q.Get("at"), body)
	default:
		http.NotFound(rw, req)
	}
}

// commandClient is the engine client, or dropClient when drop=1.
func (m *middleware) commandClient(req *http.Request) (*simpleredis.SimpleRedis, error) {
	if req.URL.Query().Get("drop") == "" {
		return m.client, nil
	}
	if m.dropClient == nil {
		return nil, fmt.Errorf("dropHost unset")
	}
	return m.dropClient, nil
}

// serveHealth Sets a unique token then Gets it into the body so compose health and handshake 502s stay one-segment.
func (m *middleware) serveHealth(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	prefix := requestKeyPrefix()
	setKey := prefix + ":set"
	if err := m.client.Set(ctx, setKey, []byte(prefix), 60); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	getValue, err := m.client.Get(ctx, setKey)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, getValue)
}

// serveGet Gets keys[0].
func (m *middleware) serveGet(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	value, err := client.Get(ctx, name)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, value)
}

// serveMGet MGets keys; missing slots are empty lines.
func (m *middleware) serveMGet(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string) {
	slots, err := client.MGet(ctx, keys)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	lines := make([]string, len(slots))
	for i, slot := range slots {
		if slot != nil {
			lines[i] = string(slot)
		}
	}
	writeOK(rw, []byte(strings.Join(lines, "\n")))
}

// serveSet Sets keys[0] to body with EX seconds.
func (m *middleware) serveSet(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, ex string, body []byte) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	seconds, ok := parseInt64Query(rw, "ex", ex)
	if !ok {
		return
	}
	if err := client.Set(ctx, name, body, seconds); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// serveDel Dels keys[0].
func (m *middleware) serveDel(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	if err := client.Del(ctx, name); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// serveIncr Incrs keys[0].
func (m *middleware) serveIncr(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	n, err := client.Incr(ctx, name)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, []byte(strconv.FormatInt(n, 10)))
}

// serveIncrBy IncrBys keys[0] by delta.
func (m *middleware) serveIncrBy(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, delta string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	n, ok := parseInt64Query(rw, "delta", delta)
	if !ok {
		return
	}
	got, err := client.IncrBy(ctx, name, n)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, []byte(strconv.FormatInt(got, 10)))
}

// serveExpire Expires keys[0] by ex seconds.
func (m *middleware) serveExpire(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, ex string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	seconds, ok := parseInt64Query(rw, "ex", ex)
	if !ok {
		return
	}
	if err := client.Expire(ctx, name, seconds); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// serveExpireAt ExpireAts keys[0] at unix seconds.
func (m *middleware) serveExpireAt(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, at string) {
	name, ok := firstKey(rw, keys)
	if !ok {
		return
	}
	unixSeconds, ok := parseInt64Query(rw, "at", at)
	if !ok {
		return
	}
	if err := client.ExpireAt(ctx, name, unixSeconds); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// serveEval Evals the request body with KEYS=keys, ARGV=args, and the caller digest.
func (m *middleware) serveEval(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys, args []string, digest string, body []byte) {
	if digest == "" {
		http.Error(rw, "digest required", http.StatusBadRequest)
		return
	}
	values, err := client.Eval(ctx, string(body), digest, keys, args)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	lines := make([]string, len(values))
	for i, slot := range values {
		lines[i] = string(slot)
	}
	writeOK(rw, []byte(strings.Join(lines, "\n")))
}

// serveMSetEX writes keys to newline-split body values with EX seconds.
func (m *middleware) serveMSetEX(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, ex string, body []byte) {
	seconds, ok := parseInt64Query(rw, "ex", ex)
	if !ok {
		return
	}
	values, ok := bodyValues(rw, keys, body)
	if !ok {
		return
	}
	if err := client.MSetEX(ctx, keys, values, seconds); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// serveMSetEXAt writes keys to newline-split body values with EXAT unix seconds.
func (m *middleware) serveMSetEXAt(rw http.ResponseWriter, ctx context.Context, client *simpleredis.SimpleRedis, keys []string, at string, body []byte) {
	unixSeconds, ok := parseInt64Query(rw, "at", at)
	if !ok {
		return
	}
	values, ok := bodyValues(rw, keys, body)
	if !ok {
		return
	}
	if err := client.MSetEXAt(ctx, keys, values, unixSeconds); err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	writeOK(rw, nil)
}

// firstKey writes 400 when key is missing.
func firstKey(rw http.ResponseWriter, keys []string) (string, bool) {
	if len(keys) == 0 || keys[0] == "" {
		http.Error(rw, "key required", http.StatusBadRequest)
		return "", false
	}
	return keys[0], true
}

// parseInt64Query writes 400 when name is missing or not an integer.
func parseInt64Query(rw http.ResponseWriter, name, raw string) (int64, bool) {
	if raw == "" {
		http.Error(rw, name+" required", http.StatusBadRequest)
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		http.Error(rw, name+" invalid", http.StatusBadRequest)
		return 0, false
	}
	return n, true
}

// bodyValues splits body on newlines; one key with no newline is the whole body.
func bodyValues(rw http.ResponseWriter, keys []string, body []byte) ([][]byte, bool) {
	if len(keys) == 0 {
		http.Error(rw, "key required", http.StatusBadRequest)
		return nil, false
	}
	if len(keys) == 1 {
		return [][]byte{body}, true
	}
	parts := strings.Split(string(body), "\n")
	if len(parts) != len(keys) {
		http.Error(rw, "value count", http.StatusBadRequest)
		return nil, false
	}
	values := make([][]byte, len(parts))
	for i, part := range parts {
		values[i] = []byte(part)
	}
	return values, true
}

// writeOK writes HTTP 200 with payload (may be empty).
func writeOK(rw http.ResponseWriter, body []byte) {
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	if len(body) > 0 {
		_, _ = rw.Write(body)
	}
}
