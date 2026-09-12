package simpleredis

import (
	"context"
	"crypto/sha1" //nolint:gosec // Redis EVALSHA digest is SHA-1
	"encoding/hex"
	"strconv"
	"strings"
)

const (
	// Redis EVAL / EVALSHA verbs and the NOSCRIPT miss prefix (after '-' is stripped).
	evalVerb       = "EVAL"
	evalShaVerb    = "EVALSHA"
	noScriptPrefix = "NOSCRIPT"
)

// Eval runs a Lua script with KEYS then ARGV. Hashes the body each call and sends EVALSHA; on NOSCRIPT falls back once to EVAL.
func (sr *SimpleRedis) Eval(script string, keys []string, args []string) ([][]byte, error) {
	return sr.EvalContext(context.Background(), script, keys, args)
}

// EvalContext runs a Lua script using ctx for cancel and deadline. EVALSHA then NOSCRIPT→EVAL share ctx.
func (sr *SimpleRedis) EvalContext(ctx context.Context, script string, keys []string, args []string) ([][]byte, error) {
	// Hash every call: SHA-1 of a limiter script (~470 B) is cheaper than a mutex, and a map of bodies would need a lock because Go maps are not concurrent.
	digest := scriptSHA1Hex(script)
	values, err := sr.exec(ctx, evalArgv(evalShaVerb, digest, keys, args)...)
	// Miss: engine has no matching digest (FLUSH, restart); EVAL is the only send of the body so the engine stores it.
	if err != nil && strings.HasPrefix(err.Error(), noScriptPrefix) {
		return sr.exec(ctx, evalArgv(evalVerb, script, keys, args)...)
	}
	return values, err
}

// scriptSHA1Hex is Redis sha1hex of the script bytes (lowercase 40-char hex). Eval hashes each call; no client digest table.
func scriptSHA1Hex(script string) string {
	sum := sha1.Sum([]byte(script)) //nolint:gosec // Redis EVALSHA digest is SHA-1
	return hex.EncodeToString(sum[:])
}

// evalArgv builds EVAL or EVALSHA argv: verb, script-or-digest, decimal numkeys, keys, then args.
func evalArgv(verb, scriptOrDigest string, keys, args []string) [][]byte {
	wire := make([][]byte, 0, 3+len(keys)+len(args))
	wire = append(wire, []byte(verb), []byte(scriptOrDigest), []byte(strconv.Itoa(len(keys))))
	for _, key := range keys {
		wire = append(wire, []byte(key))
	}
	for _, arg := range args {
		wire = append(wire, []byte(arg))
	}
	return wire
}
