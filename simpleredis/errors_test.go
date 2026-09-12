package simpleredis

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelsMatchThroughWrap(t *testing.T) {
	type row struct {
		name      string
		sentinel  error
		token     string
		predicate func(error) bool
	}
	rows := []row{
		{"miss", ErrMiss, RedisMiss, IsMiss},
		{"unreachable", ErrUnreachable, RedisUnreachable, IsUnreachable},
		{"timeout", ErrTimeout, RedisTimeout, nil},
		{"noauth", ErrNoAuth, RedisNoAuth, nil},
		{"issue", ErrIssue, RedisIssue, nil},
		{"poolwait", ErrPoolWait, RedisUnreachable, IsPoolWait},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			wrapped := fmt.Errorf("context: %w", row.sentinel)
			if !errors.Is(wrapped, row.sentinel) {
				t.Fatalf("errors.Is(wrapped, sentinel) = false")
			}
			if wrapped.Error() == row.token {
				t.Fatalf("wrapped.Error() still equals token %q", row.token)
			}
			if row.predicate != nil && !row.predicate(wrapped) {
				t.Fatal("predicate(wrapped) = false")
			}
		})
	}
}

func TestPoolWaitWrapsUnreachableAndIsNotRetried(t *testing.T) {
	if !errors.Is(ErrPoolWait, ErrUnreachable) {
		t.Fatal("errors.Is(ErrPoolWait, ErrUnreachable) = false")
	}
	if !IsPoolWait(ErrPoolWait) {
		t.Fatal("IsPoolWait(ErrPoolWait) = false")
	}
	if IsPoolWait(ErrUnreachable) {
		t.Fatal("IsPoolWait(ErrUnreachable) = true")
	}
	if shouldRetry(ErrPoolWait) {
		t.Fatal("shouldRetry(ErrPoolWait) = true, want false so MaxRetries does not multiply PoolTimeout")
	}
	if !shouldRetry(ErrUnreachable) {
		t.Fatal("shouldRetry(ErrUnreachable) = false, want true")
	}
}
