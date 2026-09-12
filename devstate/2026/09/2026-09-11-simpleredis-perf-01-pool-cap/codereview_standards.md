# Standards

1. [hard] Leave a trail — `e2e/simpleredisprobe/plugin.go:1` — package and handler comments still say Init/Inited after `New` switched to `simpleredis.New`
   ```
   // Package simpleredisprobe is a Traefik local plugin that Inits SimpleRedis
   // middleware holds the Inited clients and the next handler in the Traefik chain.
   // New Inits SimpleRedis from Config and returns a handler. It does not dial.
   ```
   → Reword to `simpleredis.New` / clients built at Traefik `New` (no `Init` API).
   Status: done
   Argument: reworded package, middleware, and New comments to simpleredis.New / construct (plugin.go); matching plugin_test.go comment.
