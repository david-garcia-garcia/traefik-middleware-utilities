package simpleredis

import (
	"context"
	"strconv"
)

// Get fetches the value for key name in redis.
func (sr *SimpleRedis) Get(name string) ([]byte, error) {
	return sr.GetContext(context.Background(), name)
}

// GetContext fetches the value for key name using ctx for cancel and deadline.
func (sr *SimpleRedis) GetContext(ctx context.Context, name string) ([]byte, error) {
	values, err := sr.exec(ctx, []byte("GET"), []byte(name))
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, errIssue
	}
	return values[0], nil
}

// MGet fetches the values for keys names in redis, nil where a key is missing.
func (sr *SimpleRedis) MGet(names []string) ([][]byte, error) {
	return sr.MGetContext(context.Background(), names)
}

// MGetContext fetches the values for keys names using ctx for cancel and deadline.
func (sr *SimpleRedis) MGetContext(ctx context.Context, names []string) ([][]byte, error) {
	if len(names) == 0 {
		return nil, nil
	}
	args := make([][]byte, 0, len(names)+1)
	args = append(args, []byte("MGET"))
	for _, name := range names {
		args = append(args, []byte(name))
	}
	values, err := sr.exec(ctx, args...)
	if err != nil {
		return nil, err
	}
	if len(values) != len(names) {
		return nil, errIssue
	}
	return values, nil
}

// Set updates the value for key name in redis with value data for duration.
func (sr *SimpleRedis) Set(name string, data []byte, duration int64) error {
	return sr.SetContext(context.Background(), name, data, duration)
}

// SetContext updates the value for key name using ctx for cancel and deadline.
func (sr *SimpleRedis) SetContext(ctx context.Context, name string, data []byte, duration int64) error {
	_, err := sr.exec(ctx, []byte("SET"), []byte(name), data, []byte("EX"), []byte(strconv.FormatInt(duration, 10)))
	return err
}

// Del removes the key name in redis.
func (sr *SimpleRedis) Del(name string) error {
	return sr.DelContext(context.Background(), name)
}

// DelContext removes the key name using ctx for cancel and deadline.
func (sr *SimpleRedis) DelContext(ctx context.Context, name string) error {
	_, err := sr.exec(ctx, []byte("DEL"), []byte(name))
	return err
}

// Incr adds one to key name and returns the integer after the increment.
func (sr *SimpleRedis) Incr(name string) (int64, error) {
	return sr.IncrContext(context.Background(), name)
}

// IncrContext adds one to key name using ctx for cancel and deadline.
func (sr *SimpleRedis) IncrContext(ctx context.Context, name string) (int64, error) {
	return parseIntegerReply(sr.exec(ctx, []byte("INCR"), []byte(name)))
}

// IncrBy adds delta to key name and returns the integer after the increment.
func (sr *SimpleRedis) IncrBy(name string, delta int64) (int64, error) {
	return sr.IncrByContext(context.Background(), name, delta)
}

// IncrByContext adds delta to key name using ctx for cancel and deadline.
func (sr *SimpleRedis) IncrByContext(ctx context.Context, name string, delta int64) (int64, error) {
	return parseIntegerReply(sr.exec(ctx, []byte("INCRBY"), []byte(name), []byte(strconv.FormatInt(delta, 10))))
}

// Expire sets a TTL in seconds on key name. Integer 0 or 1 is success.
func (sr *SimpleRedis) Expire(name string, seconds int64) error {
	return sr.ExpireContext(context.Background(), name, seconds)
}

// ExpireContext sets a TTL in seconds on key name using ctx for cancel and deadline.
func (sr *SimpleRedis) ExpireContext(ctx context.Context, name string, seconds int64) error {
	_, err := sr.exec(ctx, []byte("EXPIRE"), []byte(name), []byte(strconv.FormatInt(seconds, 10)))
	return err
}

// ExpireAt sets an absolute Unix expiry on key name. Integer 0 or 1 is success.
func (sr *SimpleRedis) ExpireAt(name string, unixSeconds int64) error {
	return sr.ExpireAtContext(context.Background(), name, unixSeconds)
}

// ExpireAtContext sets an absolute Unix expiry on key name using ctx for cancel and deadline.
func (sr *SimpleRedis) ExpireAtContext(ctx context.Context, name string, unixSeconds int64) error {
	_, err := sr.exec(ctx, []byte("EXPIREAT"), []byte(name), []byte(strconv.FormatInt(unixSeconds, 10)))
	return err
}

// parseIntegerReply reads one decimal integer from a : reply. Garbage payload is redis:issue?.
func parseIntegerReply(values [][]byte, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if len(values) != 1 {
		return 0, errIssue
	}
	n, convErr := strconv.ParseInt(string(values[0]), 10, 64)
	if convErr != nil {
		return 0, errIssue
	}
	return n, nil
}
