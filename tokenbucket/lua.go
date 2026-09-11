package tokenbucket

// Copyright Containous SAS / Traefik Labs. MIT.
// Copied from traefik/traefik pkg/middlewares/ratelimiter/lua.go @903e8a9.
// table.maxn replaced with #rl_source == 4 (Dragonfly Lua 5.4).

// allowTokenBucketScript is Traefik AllowTokenBucketRaw. KEYS[1] is the hash.
const allowTokenBucketScript = `
local key = KEYS[1]
local limit, burst, ttl, t, max_delay = tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4]),
	tonumber(ARGV[5])

local bucket = {
	limit = limit,
	burst = burst,
	tokens = 0,
	last = 0
}

local rl_source = redis.call('hgetall', key)

if #rl_source == 4 then
	bucket.last = tonumber(rl_source[2])
	bucket.tokens = tonumber(rl_source[4])
end

local last = bucket.last
if t < last then
	last = t
end

local elapsed = t - last
local delta = bucket.limit * elapsed
local tokens = bucket.tokens + delta
tokens = math.min(tokens, bucket.burst)
tokens = tokens - 1

local wait_duration = 0
if tokens < 0 then
	wait_duration = (tokens * -1) / bucket.limit
	if wait_duration > max_delay then
		tokens = tokens + 1
		tokens = math.min(tokens, burst)
	end
end

redis.call('hset', key, 'last', t, 'tokens', tokens)
redis.call('expire', key, ttl)

return {tostring(true), tostring(wait_duration), tostring(tokens)}
`
