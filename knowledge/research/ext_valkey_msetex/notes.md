# Valkey MSETEX

Official Valkey command: atomically set many string keys with one shared expiration. Shipped in **Valkey 9.1.0**. ([valkey.io MSETEX](https://valkey.io/commands/msetex/), [.sources/msetex.md](.sources/msetex.md))

## Argv

```
MSETEX numkeys key value [key value ...] [NX | XX]
  [EX seconds | PX milliseconds | EXAT unix-time-seconds |
   PXAT unix-time-milliseconds | KEEPTTL]
```

- `numkeys` is the **number of keys** (pair count), not the total token count.
- Pairs follow immediately: `k1 v1 k2 v2 …`.
- Expiration options (`EX` / `PX` / `EXAT` / `PXAT` / `KEEPTTL`) are mutually exclusive.
- `NX` and `XX` are mutually exclusive. This product's Go API does not send NX/XX.

Example from the official page: `MSETEX 2 key1 "Hello" key2 "World" EX 10` → integer `1`, then `TTL key1` is `10`. ([valkey.io MSETEX](https://valkey.io/commands/msetex/), [.sources/msetex.md](.sources/msetex.md))

Omitting every expiration option is legal on the wire (same foot-gun as `MSET` stripping TTLs). A wrapper that requires `EX` or `EXAT` avoids that.

The official page does **not** document past `EXAT` / `PXAT`. Delete-on-past is documented for `EXPIREAT` (`ext_redis_expire`), not on this page.

## Reply

RESP2/RESP3 integer:

| Reply | Meaning |
|-------|---------|
| `1` | All keys were set |
| `0` | No key was set (`NX` when at least one key existed, or `XX` when at least one key was missing) |

Without NX/XX, `0` is not a documented success path. ([valkey.io MSETEX — RESP2/RESP3 Reply](https://valkey.io/commands/msetex/), [.sources/msetex.md](.sources/msetex.md))
