package tokenbucket

// Live engine repro for traefik/traefik#14297.
// Gated like TestLive_Limiter: skipped under -short or when no
// TOKENBUCKET_LIVE_<ENGINE> is set.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/simpleredis"
)

// luaUnpack14297 works on Redis (Lua 5.1 unpack) and Dragonfly (Lua 5.4 table.unpack).
const luaUnpack14297 = "local u = table.unpack or unpack\n"

// liveKey14297 is a per-run, per-test Redis key for issue 14297.
func liveKey14297(t *testing.T, suffix string) string {
	t.Helper()
	return "tb-14297:" + strconv.FormatInt(time.Now().UnixNano(), 36) + ":" + t.Name() + ":" + suffix
}

// evalKey14297 runs a test script on KEYS[1]. The script touches only that key.
func evalKey14297(t *testing.T, client *simpleredis.SimpleRedis, script, key string, args ...string) [][]byte {
	t.Helper()
	values, err := client.Eval(context.Background(), script, simpleredis.ScriptSHA1Hex(script), []string{key}, args)
	if err != nil {
		t.Fatalf("eval on %s: %v", key, err)
	}
	return values
}

// hsetKey14297 writes field/value pairs to KEYS[1] in the order given.
func hsetKey14297(t *testing.T, client *simpleredis.SimpleRedis, key string, pairs ...string) {
	t.Helper()
	evalKey14297(t, client, luaUnpack14297+"redis.call('hset', KEYS[1], u(ARGV)) return 1", key, pairs...)
}

// hdelKey14297 removes fields from KEYS[1].
func hdelKey14297(t *testing.T, client *simpleredis.SimpleRedis, key string, fields ...string) {
	t.Helper()
	evalKey14297(t, client, luaUnpack14297+"redis.call('hdel', KEYS[1], u(ARGV)) return 1", key, fields...)
}

// hgetallFieldOrder14297 returns the field names of KEYS[1] in HGETALL order.
func hgetallFieldOrder14297(t *testing.T, client *simpleredis.SimpleRedis, key string) []string {
	t.Helper()
	values := evalKey14297(t, client, "return redis.call('hgetall', KEYS[1])", key)
	var fields []string
	for i := 0; i+1 < len(values); i += 2 {
		fields = append(fields, string(values[i]))
	}
	return fields
}

// countAdmits14297 calls allow up to attempts times and returns how many were admitted before the first deny.
func countAdmits14297(t *testing.T, attempts int, allow func() (bool, error)) int {
	t.Helper()
	admitted := 0
	for i := 0; i < attempts; i++ {
		ok, err := allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i+1, err)
		}
		if !ok {
			return admitted
		}
		admitted++
	}
	return admitted
}

// TestTraefik14297_HGetAllFieldOrder: traefik/traefik#14297.
// A correct limiter admits burst and denies the rest when the stored bucket is already full
// (last = now, tokens = burst), regardless of hash field order.
//   - control_last_first: fields in the order the script writes them.
//   - tokens_first_listpack: same fields inserted tokens-first; listpack keeps insertion order.
//   - tokens_first_hashtable: a long field forces hashtable encoding and is then removed.
//     Hashtable order depends on the per-process hash seed, so the test tries table sizes until
//     HGETALL returns tokens first, and skips if none does. That skip is expected.
func TestTraefik14297_HGetAllFieldOrder(t *testing.T) {
	runForEachLiveEngine(t, func(t *testing.T, addr string) {
		t.Helper()
		client := waitLiveClient(t, addr)
		const burst = int64(5)
		now := time.Unix(time.Now().Unix(), 0)
		nowMicro := strconv.FormatInt(now.UnixMicro(), 10)
		burstText := strconv.FormatInt(burst, 10)

		assertLimits14297 := func(t *testing.T, key string) {
			t.Helper()
			t.Logf("HGETALL field order before Allow: %v", hgetallFieldOrder14297(t, client, key))
			limiter, err := NewRedis(client, 1.0/60, burst, 0, 120*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			limiter.SetNowForTest(func() time.Time { return now })
			got := countAdmits14297(t, int(burst)+5, func() (bool, error) {
				status, allowErr := limiter.Allow(context.Background(), key)
				if allowErr != nil {
					return false, allowErr
				}
				return status.Allowed, nil
			})
			after := evalKey14297(t, client, "return redis.call('hmget', KEYS[1], 'last', 'tokens')", key)
			t.Logf("admitted %d of %d; stored last=%s tokens=%s; HGETALL field order after Allow: %v", got, burst+5, after[0], after[1], hgetallFieldOrder14297(t, client, key))
			if got != int(burst) {
				t.Fatalf("admitted %d of %d calls, want burst %d", got, burst+5, burst)
			}
		}

		t.Run("control_last_first", func(t *testing.T) {
			key := liveKey14297(t, "bucket")
			hsetKey14297(t, client, key, "last", nowMicro, "tokens", burstText)
			assertLimits14297(t, key)
		})
		t.Run("tokens_first_listpack", func(t *testing.T) {
			key := liveKey14297(t, "bucket")
			hsetKey14297(t, client, key, "tokens", burstText, "last", nowMicro)
			if order := hgetallFieldOrder14297(t, client, key); len(order) != 2 || order[0] != "tokens" {
				t.Skipf("engine did not keep insertion order (%v); condition not reached", order)
			}
			assertLimits14297(t, key)
		})
		t.Run("tokens_first_hashtable", func(t *testing.T) {
			long := strings.Repeat("x", 80) // above hash-max-listpack-value (64) on Redis
			for fillers := 0; fillers <= 64; fillers++ {
				key := liveKey14297(t, "bucket-"+strconv.Itoa(fillers))
				pairs := []string{"pad", long}
				names := []string{"pad"}
				for i := 0; i < fillers; i++ {
					name := "f" + strconv.Itoa(i)
					pairs = append(pairs, name, "1")
					names = append(names, name)
				}
				hsetKey14297(t, client, key, pairs...)
				hsetKey14297(t, client, key, "last", nowMicro, "tokens", burstText)
				hdelKey14297(t, client, key, names...)
				// HDEL shrinks the table with an incremental rehash that advances one step per lookup.
				// Until it ends, HGETALL order can still change. Finish it before reading the order.
				evalKey14297(t, client, "for i = 1, 512 do redis.call('hget', KEYS[1], 'last') end redis.call('expire', KEYS[1], 120) return 1", key)
				order := hgetallFieldOrder14297(t, client, key)
				if len(order) == 2 && order[0] == "tokens" {
					t.Logf("hashtable-encoded bucket returns tokens first after %d filler fields", fillers)
					assertLimits14297(t, key)
					return
				}
			}
			t.Skip("no table size made HGETALL return tokens first on this server process; condition not reached")
		})
	})
}
