// Package ratelimitprobe is a Traefik local plugin that runs tokenbucket.Redis or windowcounter.Limiter on each request,
// writes the Status as rate-limit headers, and forwards admitted requests to the next handler. Pester drives it to prove
// both limiters load under Traefik's own Yaegi and that two middleware instances share one budget through the engine.
package ratelimitprobe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter"
)

const (
	defaultHost = "redis:6379"
	// keyHeader picks the limiter key so each Pester case owns its budget; requests without it share defaultKey.
	keyHeader  = "X-Probe-Key"
	defaultKey = "default"
	// deniedBody is the 429 body Pester asserts.
	deniedBody = "ratelimit:denied"

	limiterTokenBucket   = "tokenbucket"
	limiterWindowCounter = "windowcounter"
)

// Config is the dynamic plugin settings Traefik decodes. Numbers and durations are text because Docker labels are text.
type Config struct {
	Host      string `json:"host,omitempty" yaml:"host,omitempty"`
	Limiter   string `json:"limiter,omitempty" yaml:"limiter,omitempty"`
	KeyPrefix string `json:"keyPrefix,omitempty" yaml:"keyPrefix,omitempty"`
	// Rate (requests per second) and Burst configure tokenbucket.
	Rate  string `json:"rate,omitempty" yaml:"rate,omitempty"`
	Burst string `json:"burst,omitempty" yaml:"burst,omitempty"`
	// Limit, Window (Go duration), and SyncRate (Go duration, empty or 0 is exact) configure windowcounter.
	Limit    string `json:"limit,omitempty" yaml:"limit,omitempty"`
	Window   string `json:"window,omitempty" yaml:"window,omitempty"`
	SyncRate string `json:"syncRate,omitempty" yaml:"syncRate,omitempty"`
}

// CreateConfig returns default plugin settings (compose Redis, tokenbucket).
func CreateConfig() *Config {
	return &Config{Host: defaultHost, Limiter: limiterTokenBucket}
}

// decision is the part of a tokenbucket or windowcounter Status the probe writes.
type decision struct {
	allowed    bool
	limit      int64
	remaining  int64
	reset      time.Duration
	retryAfter time.Duration
}

// takeFunc is one limiter decision for key.
type takeFunc func(ctx context.Context, key string) (decision, error)

// middleware holds the limiter built in New.
type middleware struct {
	next      http.Handler
	keyPrefix string
	take      takeFunc
}

// New builds SimpleRedis and the configured limiter. It does not dial. An empty keyPrefix is the middleware name.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	_ = ctx
	if next == nil {
		return nil, errors.New("ratelimitprobe: Traefik next handler is required")
	}
	if cfg == nil {
		cfg = CreateConfig()
	}
	host := cfg.Host
	if host == "" {
		host = defaultHost
	}
	keyPrefix := cfg.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = name
	}
	client, err := simpleredis.New(simpleredis.Config{Host: host})
	if err != nil {
		return nil, err
	}
	var take takeFunc
	switch cfg.Limiter {
	case limiterTokenBucket, "":
		take, err = newTokenBucket(client, cfg)
	case limiterWindowCounter:
		take, err = newWindowCounter(client, cfg)
	default:
		err = fmt.Errorf("ratelimitprobe: unknown limiter %q", cfg.Limiter)
	}
	if err != nil {
		client.Close()
		return nil, err
	}
	return &middleware{next: next, keyPrefix: keyPrefix, take: take}, nil
}

// newTokenBucket builds tokenbucket.Redis with no admit delay, so a request is either served now or denied.
func newTokenBucket(client *simpleredis.SimpleRedis, cfg *Config) (takeFunc, error) {
	rate, err := strconv.ParseFloat(cfg.Rate, 64)
	if err != nil {
		return nil, fmt.Errorf("ratelimitprobe: rate: %w", err)
	}
	burst, err := strconv.ParseInt(cfg.Burst, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ratelimitprobe: burst: %w", err)
	}
	limiter, err := tokenbucket.NewRedis(client, rate, burst, 0, time.Second)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, key string) (decision, error) {
		status, err := limiter.Allow(ctx, key)
		if err != nil {
			return decision{}, err
		}
		return decision{
			allowed:    status.Allowed,
			limit:      status.Limit,
			remaining:  status.Remaining,
			reset:      status.Reset,
			retryAfter: status.RetryAfter,
		}, nil
	}, nil
}

// newWindowCounter builds windowcounter.Limiter; Take counts one hit per request.
func newWindowCounter(client *simpleredis.SimpleRedis, cfg *Config) (takeFunc, error) {
	limit, err := strconv.ParseInt(cfg.Limit, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ratelimitprobe: limit: %w", err)
	}
	window, err := time.ParseDuration(cfg.Window)
	if err != nil {
		return nil, fmt.Errorf("ratelimitprobe: window: %w", err)
	}
	syncRate := time.Duration(0)
	if cfg.SyncRate != "" {
		syncRate, err = time.ParseDuration(cfg.SyncRate)
		if err != nil {
			return nil, fmt.Errorf("ratelimitprobe: syncRate: %w", err)
		}
	}
	limiter, err := windowcounter.New(client, syncRate)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, key string) (decision, error) {
		status, err := limiter.Take(ctx, key, limit, window)
		if err != nil {
			return decision{}, err
		}
		return decision{
			allowed:    status.Allowed,
			limit:      status.Limit,
			remaining:  status.Remaining,
			reset:      status.Reset,
			retryAfter: status.RetryAfter,
		}, nil
	}, nil
}

// ServeHTTP takes one hit on keyPrefix:<X-Probe-Key>, writes X-RateLimit-*, then forwards, answers 429, or answers 502 on a
// limiter error.
func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	key := req.Header.Get(keyHeader)
	if key == "" {
		key = defaultKey
	}
	result, err := m.take(req.Context(), m.keyPrefix+":"+key)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}
	header := rw.Header()
	header.Set("X-RateLimit-Limit", strconv.FormatInt(result.limit, 10))
	header.Set("X-RateLimit-Remaining", strconv.FormatInt(result.remaining, 10))
	header.Set("X-RateLimit-Reset", strconv.FormatInt(ceilSeconds(result.reset), 10))
	if !result.allowed {
		retryAfter := ceilSeconds(result.retryAfter)
		if retryAfter < 1 {
			retryAfter = 1
		}
		header.Set("Retry-After", strconv.FormatInt(retryAfter, 10))
		http.Error(rw, deniedBody, http.StatusTooManyRequests)
		return
	}
	m.next.ServeHTTP(rw, req)
}

// ceilSeconds is d rounded up to whole seconds, zero when d is not positive. It does not add before dividing, so a saturated
// duration does not overflow.
func ceilSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	seconds := int64(d / time.Second)
	if d%time.Second != 0 {
		seconds++
	}
	return seconds
}
