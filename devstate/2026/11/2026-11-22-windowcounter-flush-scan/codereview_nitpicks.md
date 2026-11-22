# Nitpicks

1. [hard] Name for the scope — `windowcounter/limiter.go:366` — `putWindowLocked`, `setExpireLocked` y `deleteWindowLocked` llaman `key` al Redis window key; en este archivo ese rol es `redisKey` y `key` es la opaque key de Take/Peek
   Quote:
   ```go
   func (l *Limiter) putWindowLocked(key string, state *windowState) {
   	l.windows[key] = state
   	bucket := l.expiry[state.expireAt]
   	if bucket == nil {
   		bucket = map[string]*windowState{}
   		l.expiry[state.expireAt] = bucket
   	}
   	bucket[key] = state
   }
   ```
   Fix: Renombrar el parámetro `key` a `redisKey` en `putWindowLocked`, `setExpireLocked` y `deleteWindowLocked`
   Status: done
   Argument: parámetro `key` → `redisKey` en los tres helpers.
2. [hard] Clear conditions — `windowcounter/limiter.go:571` — el prune salta cubos con el complemento `expireAt != 0 && nowUnix < expireAt` en vez de nombrar los due (`expireAt == 0 || nowUnix >= expireAt`)
   Quote:
   ```go
   for expireAt, bucket := range l.expiry {
   	if expireAt != 0 && nowUnix < expireAt {
   		continue
   	}
   	for redisKey, state := range bucket {
   		if state.localDelta == 0 && state.flushDelta == 0 {
   			l.deleteWindowLocked(redisKey, state)
   		}
   	}
   }
   ```
   Fix: Nombrar el conjunto due (`expireAt == 0 || nowUnix >= expireAt`) y hacer `continue` cuando no se cumpla
   Status: done
   Argument: `due := expireAt == 0 || nowUnix >= expireAt`; `continue` si no.
