package leakybucket

// pourScript leaks HASH water/last once, then adds ARGV delta when it would not exceed capacity.
// KEYS[1] is the hash. table.maxn is not used (#hash == 4 for Dragonfly Lua 5.4).
const pourScript = `
local key = KEYS[1]
local leak, capacity, ttl, now, delta = tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4]),
	tonumber(ARGV[5])

local water = 0
local last = 0
local hash = redis.call('hgetall', key)
if #hash == 4 then
	last = tonumber(hash[2])
	water = tonumber(hash[4])
end

if now < last then
	last = now
end

local elapsed = (now - last) / 1e6
water = water - leak * elapsed
if water < 0 then
	water = 0
end

local allowed = 1
if delta > 0 and (water + delta) > capacity then
	allowed = 0
else
	water = water + delta
end

redis.call('hset', key, 'last', now, 'water', water)
redis.call('expire', key, ttl)

local until_not_full = 0
if water + 1 > capacity then
	until_not_full = (water - (capacity - 1)) / leak * 1e6
end

return {tostring(allowed), tostring(water), tostring(until_not_full)}
`
