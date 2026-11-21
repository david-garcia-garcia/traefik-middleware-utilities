package simpleredis

import (
	"context"
	"errors"
	"testing"
)

// TestEvalArgvAndIntegerReply is dest NOSCRIPT success: EVALSHA miss then EVAL of the body.
// Those are two exec hops. Each hop binds its own overall deadline on purpose so EVAL still has a full command budget after EVALSHA.
// Do not assert elapsed against one public-command budget; sharing remaining time can starve EVAL.
func TestEvalArgvAndIntegerReply(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})

	values, err := redis.Eval(context.Background(), kongIncrbyExpireatScript, ScriptSHA1Hex(kongIncrbyExpireatScript), []string{"win"}, []string{"7", "1700000000"})
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "7" {
		t.Fatalf("Eval = %q, want [7]", values)
	}
	got := fake.lastEvalCommand()
	if len(got) != 6 || got[0] != evalVerb || got[1] != kongIncrbyExpireatScript || got[2] != "1" || got[3] != "win" || got[4] != "7" || got[5] != "1700000000" {
		t.Fatalf("fallback EVAL argv = %v", got)
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 1 || eval != 1 {
		t.Fatalf("after first Eval EVALSHA=%d EVAL=%d, want 1, 1", evalSha, eval)
	}
}

// TestEvalNoscriptFallbackIsOwnExec documents the intended per-hop budget: EVALSHA then EVAL both run, and Eval succeeds.
// Do not add a first-hop delay then stall, and do not fail because elapsed exceeds one public-command budget.
func TestEvalNoscriptFallbackIsOwnExec(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})
	if _, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("Eval NOSCRIPT fallback: %v", err)
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 1 || eval != 1 {
		t.Fatalf("EVALSHA=%d EVAL=%d, want 1, 1", evalSha, eval)
	}
}

func TestEvalLaterSendsEvalShaNotBody(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})

	if _, err := redis.Eval(context.Background(), kongIncrbyExpireatScript, ScriptSHA1Hex(kongIncrbyExpireatScript), []string{"win"}, []string{"7", "1700000000"}); err != nil {
		t.Fatalf("first Eval: %v", err)
	}
	values, err := redis.Eval(context.Background(), kongIncrbyExpireatScript, ScriptSHA1Hex(kongIncrbyExpireatScript), []string{"win2"}, []string{"7", "1700000000"})
	if err != nil {
		t.Fatalf("second Eval: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "7" {
		t.Fatalf("second Eval = %q, want [7]", values)
	}
	got := fake.lastEvalCommand()
	digest := ScriptSHA1Hex(kongIncrbyExpireatScript)
	if len(got) != 6 || got[0] != evalShaVerb || got[1] != digest || got[2] != "1" || got[3] != "win2" || got[4] != "7" || got[5] != "1700000000" {
		t.Fatalf("second argv = %v, want EVALSHA %s 1 win2 7 1700000000", got, digest)
	}
	for _, arg := range got {
		if arg == kongIncrbyExpireatScript {
			t.Fatalf("second argv included the script body: %v", got)
		}
	}
	evalSha, eval := fake.evalCommandCounts()
	if evalSha != 2 || eval != 1 {
		t.Fatalf("after second Eval EVALSHA=%d EVAL=%d, want 2, 1", evalSha, eval)
	}
}

func TestEvalTwoScriptsTwoDigests(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})

	if _, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("script A first: %v", err)
	}
	if _, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("script A second: %v", err)
	}
	argvA := fake.lastEvalCommand()
	if len(argvA) < 2 || argvA[0] != evalShaVerb {
		t.Fatalf("script A second argv = %v, want EVALSHA", argvA)
	}
	if _, err := redis.Eval(context.Background(), "return 2", ScriptSHA1Hex("return 2"), nil, nil); err != nil {
		t.Fatalf("script B first: %v", err)
	}
	if _, err := redis.Eval(context.Background(), "return 2", ScriptSHA1Hex("return 2"), nil, nil); err != nil {
		t.Fatalf("script B second: %v", err)
	}
	argvB := fake.lastEvalCommand()
	if len(argvB) < 2 || argvB[0] != evalShaVerb {
		t.Fatalf("script B second argv = %v, want EVALSHA", argvB)
	}
	if argvA[1] == argvB[1] {
		t.Fatalf("scripts A and B shared digest %q", argvA[1])
	}
}

func TestEvalEmptyKeysSendsNumkeysZero(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})

	if _, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("first Eval: %v", err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 3 || got[0] != evalVerb || got[2] != "0" {
		t.Fatalf("fallback EVAL argv = %v, want EVAL … 0", got)
	}
	if _, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil); err != nil {
		t.Fatalf("second Eval: %v", err)
	}
	got = fake.lastEvalCommand()
	if len(got) < 3 || got[0] != evalShaVerb || got[2] != "0" {
		t.Fatalf("EVALSHA argv = %v, want EVALSHA … 0", got)
	}
}

func TestEvalEmptyKeys(t *testing.T) {
	addr := startStaticRedis(t, ":1\r\n")
	redis := newTestRedis(t, Config{Host: addr})
	values, err := redis.Eval(context.Background(), "return 1", ScriptSHA1Hex("return 1"), nil, nil)
	if err != nil {
		t.Fatalf("Eval empty: %v", err)
	}
	if len(values) != 1 || string(values[0]) != "1" {
		t.Fatalf("Eval empty = %q, want [1]", values)
	}
}

func TestScriptSHA1HexIsLowercase40(t *testing.T) {
	digest := ScriptSHA1Hex("return 1")
	if len(digest) != 40 {
		t.Fatalf("len(%q) = %d, want 40", digest, len(digest))
	}
	for _, c := range digest {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("digest %q is not lowercase hex", digest)
		}
	}
}

func TestEvalUsesCallerDigest(t *testing.T) {
	fake, addr := startFakeRedis(t, map[string]string{})
	redis := newTestRedis(t, Config{Host: addr})
	script := "return 1"
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if digest == ScriptSHA1Hex(script) {
		t.Fatal("caller digest collided with ScriptSHA1Hex(script)")
	}
	fake.loadScriptDigestForTest(digest, script)
	if _, err := redis.Eval(context.Background(), script, digest, nil, nil); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	got := fake.lastEvalCommand()
	if len(got) < 2 || got[0] != evalShaVerb || got[1] != digest {
		t.Fatalf("EVALSHA argv = %v, want caller digest %s", got, digest)
	}
	if got[1] == ScriptSHA1Hex(script) {
		t.Fatalf("EVALSHA used ScriptSHA1Hex(script), Eval hashed")
	}
}

func TestEvalNullBulkIsNotMiss(t *testing.T) {
	addr := startStaticRedis(t, "$-1\r\n")
	client := newTestRedis(t, Config{Host: addr, MaxRetries: -1})
	script := "return false"
	values, err := client.Eval(context.Background(), script, ScriptSHA1Hex(script), nil, nil)
	if errors.Is(err, ErrMiss) {
		t.Fatalf("Eval $-1: values=%q err=%v, want a result slot not redis:miss", values, err)
	}
	if err != nil {
		t.Fatalf("Eval $-1: err=%v, want nil error", err)
	}
	if len(values) != 1 || values[0] != nil {
		t.Fatalf("Eval $-1: values=%q, want one nil slot", values)
	}
}
