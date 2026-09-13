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

// ScriptSHA1Hex is Redis sha1hex of the script bytes (lowercase 40-char hex). Callers that reuse a script compute this once at init.
func ScriptSHA1Hex(script string) string {
	sum := sha1.Sum([]byte(script)) //nolint:gosec // Redis EVALSHA digest is SHA-1
	return hex.EncodeToString(sum[:])
}

// Eval runs a Lua script with KEYS then ARGV. Sends EVALSHA of the caller digest; on NOSCRIPT falls back once to EVAL of the body.
// Each hop is its own exec and binds its own overall deadline on purpose.
// The EVAL fallback must still have a full command budget after EVALSHA; do not share remaining time across hops (that can starve EVAL).
// The reply is a flat array of bulk strings or integers; Lua authors wrap each slot with tostring. Nested tables and {err=...} inside an array are redis:unsupported-reply. Top-level Lua false/nil is a nil slot, not redis:miss.
func (sr *SimpleRedis) Eval(ctx context.Context, script string, digest string, keys []string, args []string) ([][]byte, error) {
	values, err := sr.exec(ctx, evalArgv(evalShaVerb, digest, keys, args)...)
	// Miss: engine has no matching digest (FLUSH, restart); EVAL is the only send of the body so the engine stores it.
	if err != nil && strings.HasPrefix(err.Error(), noScriptPrefix) {
		sr.logDebugNoScript(ctx)
		return sr.exec(ctx, evalArgv(evalVerb, script, keys, args)...)
	}
	return values, err
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
