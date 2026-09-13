package simpleredis

import (
	"context"
	"strconv"
	"strings"
)

const maxMSetEXPairs = 1024

// groupWritePath is whether this client sends native MSETEX or the Lua fallback.
type groupWritePath int

const (
	groupWriteUnknown groupWritePath = iota
	groupWriteNative
	groupWriteLua
)

// msetexFallbackScript sets each KEYS[i] to ARGV[i] with EX or EXAT from the last two ARGV.
// Lua 5.1-safe: numeric for, no unpack / table.unpack / table.maxn. Keys stay in KEYS for Dragonfly.
const msetexFallbackScript = `local token = ARGV[#ARGV - 1]
local ttl = ARGV[#ARGV]
for i = 1, #KEYS do
  redis.call('SET', KEYS[i], ARGV[i], token, ttl)
end
return 1`

// msetexFallbackDigest is Redis sha1hex of msetexFallbackScript, computed once at package init.
var msetexFallbackDigest = ScriptSHA1Hex(msetexFallbackScript)

// MSetEX writes names and values with one shared TTL in seconds (native MSETEX or Lua fallback).
// Native MSETEX is its own exec. Unknown-command then Eval starts with a full command budget (one or two further hops).
// Each hop binds its own overall deadline on purpose; do not share remaining time across hops (that can starve EVAL).
func (sr *SimpleRedis) MSetEX(ctx context.Context, names []string, values [][]byte, seconds int64) error {
	return sr.msetex(ctx, names, values, "EX", seconds)
}

// MSetEXAt writes names and values with one shared Unix expiry (native MSETEX or Lua fallback).
// Native MSETEX is its own exec. Unknown-command then Eval starts with a full command budget (one or two further hops).
// Each hop binds its own overall deadline on purpose; do not share remaining time across hops (that can starve EVAL).
func (sr *SimpleRedis) MSetEXAt(ctx context.Context, names []string, values [][]byte, unixSeconds int64) error {
	return sr.msetex(ctx, names, values, "EXAT", unixSeconds)
}

// msetex validates the pair lists then sends native MSETEX, falling back to Eval on unknown-command.
// Native MSETEX is its own exec. The Eval fallback must still have a full command budget after unknown-command.
// Do not share remaining time across hops (that can starve EVAL).
func (sr *SimpleRedis) msetex(ctx context.Context, names []string, values [][]byte, expireToken string, ttl int64) error {
	if len(names) == 0 || len(names) != len(values) || len(names) > maxMSetEXPairs {
		return errIssue
	}
	if sr.cachedGroupWrite() == groupWriteLua {
		return sr.msetexEval(ctx, names, values, expireToken, ttl)
	}
	n, err := parseIntegerReply(sr.exec(ctx, msetexArgs(names, values, expireToken, ttl)...))
	if unknownCommand(err) {
		sr.storeGroupWrite(groupWriteLua)
		return sr.msetexEval(ctx, names, values, expireToken, ttl)
	}
	if err == nil {
		sr.storeGroupWrite(groupWriteNative)
	}
	return msetexSuccess(n, err)
}

// msetexEval runs the fallback script with names in KEYS and values then token then TTL in ARGV.
func (sr *SimpleRedis) msetexEval(ctx context.Context, names []string, values [][]byte, expireToken string, ttl int64) error {
	argv := make([]string, 0, len(values)+2)
	for _, value := range values {
		argv = append(argv, string(value))
	}
	argv = append(argv, expireToken, strconv.FormatInt(ttl, 10))
	return msetexSuccess(parseIntegerReply(sr.Eval(ctx, msetexFallbackScript, msetexFallbackDigest, names, argv)))
}

// cachedGroupWrite returns the capability cache. Callers must not hold groupWriteMu.
func (sr *SimpleRedis) cachedGroupWrite() groupWritePath {
	sr.groupWriteMu.Lock()
	defer sr.groupWriteMu.Unlock()
	return sr.groupWrite
}

// storeGroupWrite records native or lua for this client. Callers must not hold groupWriteMu.
func (sr *SimpleRedis) storeGroupWrite(path groupWritePath) {
	sr.groupWriteMu.Lock()
	sr.groupWrite = path
	sr.groupWriteMu.Unlock()
	switch path {
	case groupWriteNative:
		sr.logDebugCapability(context.Background(), capabilityNative)
	case groupWriteLua:
		sr.logDebugCapability(context.Background(), capabilityLua)
	}
}

// msetexArgs is native MSETEX: numkeys, pairs in order, then EX or EXAT, then the decimal TTL.
func msetexArgs(names []string, values [][]byte, expireToken string, ttl int64) [][]byte {
	args := make([][]byte, 0, 4+2*len(names))
	args = append(args, []byte("MSETEX"), []byte(strconv.Itoa(len(names))))
	for i, name := range names {
		args = append(args, []byte(name), values[i])
	}
	args = append(args, []byte(expireToken), []byte(strconv.FormatInt(ttl, 10)))
	return args
}

// msetexSuccess is nil when the engine returned integer 1. Other integers are redis:issue?.
func msetexSuccess(n int64, err error) error {
	if err != nil {
		return err
	}
	if n != 1 {
		return errIssue
	}
	return nil
}

// unknownCommand is true when Redis or Dragonfly rejected the verb as not in the command table.
func unknownCommand(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "ERR unknown command")
}
