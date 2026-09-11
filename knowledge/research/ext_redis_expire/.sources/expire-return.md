---
url: https://redis.io/docs/latest/commands/expire/
title: EXPIRE — Return information
fetched: 2026-09-11
authority: official
---

Return information (RESP2/RESP3), one of:
- Integer reply: 0 if the timeout was not set; for example, the key doesn't exist, or the operation was skipped because of the provided arguments.
- Integer reply: 1 if the timeout was set.

Optional flags (mutually exclusive): NX, XX, GT, LT.
