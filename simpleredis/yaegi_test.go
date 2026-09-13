package simpleredis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestYaegi_StrayExtraReplyOwnKey proves interpreted Get against a compiled stray-extra fake. Traefik is not started.
func TestYaegi_StrayExtraReplyOwnKey(t *testing.T) {
	_, addr := startStrayExtraReplyFake(t, 5)
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.GetOwnKeys(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi stray extra: %q, want ok", got)
	}
}

// TestYaegi_NewGetSetDel proves interpreted code can New, Set, Get, and Del
// against a compiled fake TCP Redis. Traefik is not started.
func TestYaegi_NewGetSetDel(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.RoundTrip(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi round-trip: %q, want ok", got)
	}
}

// TestYaegi_IncrAndEval proves interpreted Incr and Eval against a compiled fake. Traefik is not started.
func TestYaegi_IncrAndEval(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.IncrAndEval(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi incr+eval: %q, want ok", got)
	}
}

// TestYaegi_EvalNoScriptFallback proves interpreted Eval recovers from a NOSCRIPT miss. Traefik is not started.
func TestYaegi_EvalNoScriptFallback(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.EvalNoScript(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi eval noscript fallback: %q, want ok", got)
	}
}

// TestYaegi_MSetEXNative proves interpreted MSetEX against a compiled fake that implements MSETEX.
func TestYaegi_MSetEXNative(t *testing.T) {
	_, addr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.MSetEXNative(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi msetex native: %q, want ok", got)
	}
}

// TestYaegi_MatchSentinels proves interpreted errors.Is and IsMiss match a wrapped ErrMiss.
func TestYaegi_MatchSentinels(t *testing.T) {
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, `clientprobe.MatchSentinels()`)
	if got != "ok" {
		t.Fatalf("yaegi match sentinels: %q, want ok", got)
	}
}

// TestYaegi_HandshakeAuthEOFMatchesUnreachable ports TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching: interpreted IsUnreachable must match AUTH peer-close, same as compiled.
func TestYaegi_HandshakeAuthEOFMatchesUnreachable(t *testing.T) {
	_, addr := startAcceptFake(t, func(_ net.Conn, reader *bufio.Reader) {
		_, _ = readCommand(reader)
	})

	compiled := New(Config{Host: addr, Pass: "secret", MaxRetries: -1})
	_, compiledErr := compiled.Get(context.Background(), "hit")
	if !IsUnreachable(compiledErr) {
		t.Fatalf("compiled control broke: IsUnreachable(%v) = false", compiledErr)
	}

	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.HandshakeAuthEOFUnreachable(%q)`, addr))
	if got != "ok" {
		t.Fatalf("interpreted IsUnreachable(handshake AUTH EOF) = %s, want ok", got)
	}
}

// TestYaegi_HandshakeAuthWrongPassMatchesNoAuth ports TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching: interpreted errors.Is must match AUTH WRONGPASS, same as compiled.
func TestYaegi_HandshakeAuthWrongPassMatchesNoAuth(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{"hit": "t"})
	fake.setHandshakeReplies("-WRONGPASS invalid password\r\n", statusOKReply)

	compiled := New(Config{Host: addr, Pass: "wrong", MaxRetries: -1})
	_, compiledErr := compiled.Get(context.Background(), "hit")
	if !errors.Is(compiledErr, ErrNoAuth) {
		t.Fatalf("compiled control broke: errors.Is(%v, ErrNoAuth) = false", compiledErr)
	}

	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.HandshakeAuthRejectNoAuth(%q)`, addr))
	if got != "ok" {
		t.Fatalf("interpreted errors.Is(handshake WRONGPASS, ErrNoAuth) = %s, want ok", got)
	}
}

// TestYaegi_MatchPackageUnreachableAndMiss proves interpreted matchers on errors the package returns (dead port and miss), not only a %w wrap.
func TestYaegi_MatchPackageUnreachableAndMiss(t *testing.T) {
	_, missAddr := startFakeRedis(t, map[string]string{})
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.DeadPortIsUnreachable(%q)`, "127.0.0.1:1"))
	if got != "ok" {
		t.Fatalf("interpreted IsUnreachable(dead port) = %s, want ok", got)
	}
	got = evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.MissingKeyIsMiss(%q)`, missAddr))
	if got != "ok" {
		t.Fatalf("interpreted IsMiss(missing key) = %s, want ok", got)
	}
}

// TestYaegi_MSetEXLua proves interpreted MSetEX falls back to EVAL and a second call skips MSETEX.
func TestYaegi_MSetEXLua(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	fake.setRejectMSetEX()
	goPath := t.TempDir()
	writeGopathSimpleredis(t, goPath)
	writeGopathClientprobe(t, goPath)

	got := evalClientprobe(t, goPath, fmt.Sprintf(`clientprobe.MSetEXLua(%q)`, addr))
	if got != "ok" {
		t.Fatalf("yaegi msetex lua: %q, want ok", got)
	}
	if fake.msetexSendCount() != 1 {
		t.Fatalf("MSETEX sends = %d, want 1", fake.msetexSendCount())
	}
}

// evalClientprobe evaluates expr in a GOPATH interp with stdlib only (no unsafe).
func evalClientprobe(t *testing.T, goPath, expr string) string {
	t.Helper()
	interpreter := interp.New(interp.Options{GoPath: goPath})
	if err := interpreter.Use(stdlib.Symbols); err != nil {
		t.Fatalf("use stdlib: %v", err)
	}
	if _, err := interpreter.Eval(`import "clientprobe"`); err != nil {
		t.Fatalf("import clientprobe: %v", err)
	}
	evaluated, err := interpreter.Eval(expr)
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return evaluated.Interface().(string)
}

// writeGopathSimpleredis copies non-test simpleredis sources into a GOPATH module tree.
func writeGopathSimpleredis(t testing.TB, goPath string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller path")
	}
	srcDir := filepath.Dir(thisFile)
	destDir := filepath.Join(goPath, "src", "github.com", "david-garcia-garcia", "traefik-middleware-utilities", "simpleredis")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destDir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
		copied++
	}
	if copied == 0 {
		t.Fatal("no simpleredis sources copied into GOPATH")
	}
}

// writeGopathFile writes one interpreted package file under GOPATH/src/<pkg>.
func writeGopathFile(t testing.TB, goPath, pkg, name, src string) {
	t.Helper()
	dir := filepath.Join(goPath, "src", pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeGopathClientprobe writes the interpreted probe package under GOPATH/src/clientprobe.
func writeGopathClientprobe(t testing.TB, goPath string) {
	t.Helper()
	writeGopathFile(t, goPath, "clientprobe", "roundtrip.go", clientprobeSrc)
}

const clientprobeSrc = `package clientprobe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// RoundTrip builds a client, Sets a key, Gets it, and Dels it.
func RoundTrip(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	if err := client.Set(context.Background(), "k", []byte("ok"), 60); err != nil {
		return "set:" + err.Error()
	}
	got, err := client.Get(context.Background(), "k")
	if err != nil {
		return "get:" + err.Error()
	}
	if err := client.Del(context.Background(), "k"); err != nil {
		return "del:" + err.Error()
	}
	return string(got)
}

// GetOwnKeys Gets k0..k11. A stray extra bulk must not surface as another key's value.
func GetOwnKeys(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, PoolSize: 1, MaxRetries: -1})
	for i := 0; i < 12; i++ {
		key := "k" + strconv.Itoa(i)
		want := "v" + strconv.Itoa(i)
		got, err := client.Get(context.Background(), key)
		if err != nil {
			continue
		}
		if string(got) != want {
			return "Get(" + key + ")=" + string(got)
		}
	}
	return "ok"
}

const kongIncrbyExpireatScript = ` + "`" + `local exists = redis.call("exists", KEYS[1])
local value = redis.call("incrby", KEYS[1], ARGV[1])
if exists == 0 then
  redis.call("expireat", KEYS[1], ARGV[2])
end
return value` + "`" + `

// IncrAndEval Incs a missing key then Evals the Kong incrby+expireat snippet.
func IncrAndEval(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	afterIncr, err := client.Incr(context.Background(), "yaegi-incr")
	if err != nil {
		return "incr:" + err.Error()
	}
	if afterIncr != 1 {
		return fmt.Sprintf("incr:%d", afterIncr)
	}
	values, err := client.Eval(context.Background(), kongIncrbyExpireatScript, simpleredis.ScriptSHA1Hex(kongIncrbyExpireatScript), []string{"yaegi-eval"}, []string{"3", "1700000000"})
	if err != nil {
		return "eval:" + err.Error()
	}
	if len(values) != 1 || string(values[0]) != "3" {
		return fmt.Sprintf("eval:%q", values)
	}
	return "ok"
}

// EvalNoScript Evals once so the compiled fake's first EVALSHA miss must fall back.
func EvalNoScript(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	values, err := client.Eval(context.Background(), kongIncrbyExpireatScript, simpleredis.ScriptSHA1Hex(kongIncrbyExpireatScript), []string{"yaegi-noscript"}, []string{"3", "1700000000"})
	if err != nil {
		return "eval:" + err.Error()
	}
	if len(values) != 1 || string(values[0]) != "3" {
		return fmt.Sprintf("eval:%q", values)
	}
	return "ok"
}

// MSetEXNative writes one pair via MSetEX against a native MSETEX fake.
func MSetEXNative(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	if err := client.MSetEX(context.Background(), []string{"yaegi-msetex"}, [][]byte{[]byte("ok")}, 60); err != nil {
		return "msetex:" + err.Error()
	}
	got, err := client.Get(context.Background(), "yaegi-msetex")
	if err != nil {
		return "get:" + err.Error()
	}
	if string(got) != "ok" {
		return "get:" + string(got)
	}
	return "ok"
}

// LiveVerbs runs New plus Get, MGet, Set, Del, Incr, IncrBy, Expire, ExpireAt, Eval, MSetEX, and MSetEXAt against a live engine.
func LiveVerbs(host, key string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	if err := client.Set(context.Background(), key, []byte("ok"), 60); err != nil {
		return "set:" + err.Error()
	}
	got, err := client.Get(context.Background(), key)
	if err != nil {
		return "get:" + err.Error()
	}
	if string(got) != "ok" {
		return "get:" + string(got)
	}
	slots, err := client.MGet(context.Background(), []string{key, key + "-missing"})
	if err != nil {
		return "mget:" + err.Error()
	}
	if len(slots) != 2 || string(slots[0]) != "ok" || slots[1] != nil {
		return fmt.Sprintf("mget:%q", slots)
	}
	if err := client.Del(context.Background(), key); err != nil {
		return "del:" + err.Error()
	}
	n, err := client.Incr(context.Background(), key+"-n")
	if err != nil {
		return "incr:" + err.Error()
	}
	if n != 1 {
		return fmt.Sprintf("incr:%d", n)
	}
	by, err := client.IncrBy(context.Background(), key+"-by", 5)
	if err != nil {
		return "incrby:" + err.Error()
	}
	if by != 5 {
		return fmt.Sprintf("incrby:%d", by)
	}
	expireKey := key + "-ex"
	if err := client.Set(context.Background(), expireKey, []byte("1"), 60); err != nil {
		return "expire-set:" + err.Error()
	}
	if err := client.Expire(context.Background(), expireKey, 90); err != nil {
		return "expire:" + err.Error()
	}
	ttlScript := "return redis.call('TTL', KEYS[1])"
	ttlDigest := simpleredis.ScriptSHA1Hex(ttlScript)
	expireTTLValues, err := client.Eval(context.Background(), ttlScript, ttlDigest, []string{expireKey}, nil)
	if err != nil {
		return "expire-ttl:" + err.Error()
	}
	if len(expireTTLValues) != 1 {
		return fmt.Sprintf("expire-ttl:%q", expireTTLValues)
	}
	expireTTL, err := strconv.ParseInt(string(expireTTLValues[0]), 10, 64)
	if err != nil {
		return "expire-ttl-parse:" + err.Error()
	}
	if expireTTL <= 60 {
		return fmt.Sprintf("expire-ttl:%d", expireTTL)
	}
	expireAtKey := key + "-exat"
	if err := client.Set(context.Background(), expireAtKey, []byte("1"), 60); err != nil {
		return "expireat-set:" + err.Error()
	}
	if err := client.ExpireAt(context.Background(), expireAtKey, time.Now().Unix()+90); err != nil {
		return "expireat:" + err.Error()
	}
	expireAtTTLValues, err := client.Eval(context.Background(), ttlScript, ttlDigest, []string{expireAtKey}, nil)
	if err != nil {
		return "expireat-ttl:" + err.Error()
	}
	if len(expireAtTTLValues) != 1 {
		return fmt.Sprintf("expireat-ttl:%q", expireAtTTLValues)
	}
	expireAtTTL, err := strconv.ParseInt(string(expireAtTTLValues[0]), 10, 64)
	if err != nil {
		return "expireat-ttl-parse:" + err.Error()
	}
	if expireAtTTL <= 60 {
		return fmt.Sprintf("expireat-ttl:%d", expireAtTTL)
	}
	values, err := client.Eval(context.Background(), "return 1", simpleredis.ScriptSHA1Hex("return 1"), nil, nil)
	if err != nil {
		return "eval:" + err.Error()
	}
	if len(values) != 1 || string(values[0]) != "1" {
		return fmt.Sprintf("eval:%q", values)
	}
	msetexKey := key + "-m"
	if err := client.MSetEX(context.Background(), []string{msetexKey}, [][]byte{[]byte("ok")}, 60); err != nil {
		return "msetex:" + err.Error()
	}
	got, err = client.Get(context.Background(), msetexKey)
	if err != nil {
		return "msetex-get:" + err.Error()
	}
	if string(got) != "ok" {
		return "msetex-get:" + string(got)
	}
	msetexAtKey := key + "-ma"
	if err := client.MSetEXAt(context.Background(), []string{msetexAtKey}, [][]byte{[]byte("ok")}, time.Now().Unix()+90); err != nil {
		return "msetexat:" + err.Error()
	}
	got, err = client.Get(context.Background(), msetexAtKey)
	if err != nil {
		return "msetexat-get:" + err.Error()
	}
	if string(got) != "ok" {
		return "msetexat-get:" + string(got)
	}
	return "ok"
}

// MSetEXLua calls MSetEX twice so a reject-MSETEX fake can prove the cache.
func MSetEXLua(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host})
	if err := client.MSetEX(context.Background(), []string{"yaegi-msetex-lua"}, [][]byte{[]byte("ok")}, 60); err != nil {
		return "first:" + err.Error()
	}
	if err := client.MSetEX(context.Background(), []string{"yaegi-msetex-lua-2"}, [][]byte{[]byte("ok")}, 60); err != nil {
		return "second:" + err.Error()
	}
	got, err := client.Get(context.Background(), "yaegi-msetex-lua")
	if err != nil {
		return "get:" + err.Error()
	}
	if string(got) != "ok" {
		return "get:" + string(got)
	}
	return "ok"
}

// MatchSentinels proves interpreted errors.Is on an exported sentinel and IsMiss.
func MatchSentinels() string {
	wrapped := fmt.Errorf("context: %w", simpleredis.ErrMiss)
	if !errors.Is(wrapped, simpleredis.ErrMiss) {
		return "errors.Is"
	}
	if !simpleredis.IsMiss(wrapped) {
		return "IsMiss"
	}
	return "ok"
}

// HandshakeAuthEOFUnreachable reports IsUnreachable for an AUTH peer-close handshake failure.
func HandshakeAuthEOFUnreachable(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, Pass: "secret", MaxRetries: -1})
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if err.Error() != simpleredis.RedisUnreachable {
		return "text:" + err.Error()
	}
	if !simpleredis.IsUnreachable(err) {
		return "IsUnreachable"
	}
	return "ok"
}

// HandshakeAuthRejectNoAuth reports errors.Is(err, ErrNoAuth) for a WRONGPASS handshake failure.
func HandshakeAuthRejectNoAuth(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, Pass: "wrong", MaxRetries: -1})
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if err.Error() != simpleredis.RedisNoAuth {
		return "text:" + err.Error()
	}
	if !errors.Is(err, simpleredis.ErrNoAuth) {
		return "errors.Is"
	}
	return "ok"
}

// DeadPortIsUnreachable reports IsUnreachable for a TCP refuse before handshake.
func DeadPortIsUnreachable(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: -1})
	_, err := client.Get(context.Background(), "hit")
	if err == nil {
		return "no-error"
	}
	if !simpleredis.IsUnreachable(err) {
		return "IsUnreachable"
	}
	return "ok"
}

// MissingKeyIsMiss reports IsMiss for a GET of a missing key.
func MissingKeyIsMiss(host string) string {
	client := simpleredis.New(simpleredis.Config{Host: host, MaxRetries: -1})
	_, err := client.Get(context.Background(), "missing-key-yaegi")
	if err == nil {
		return "no-error"
	}
	if !simpleredis.IsMiss(err) {
		return "IsMiss"
	}
	return "ok"
}
`
