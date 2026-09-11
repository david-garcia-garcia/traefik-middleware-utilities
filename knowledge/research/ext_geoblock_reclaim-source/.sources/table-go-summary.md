---
ref: david-garcia-garcia/traefik-geoblock@22f09a0:pkg/reclaim/table.go
title: reclaim.Table implementation summary
fetched: 2026-09-11
authority: source
---

Table stores one value per key as any. States: slotBusy, slotAwake, slotAsleep, slotGone.
Open registers key before create so concurrent first Opens run create once.
Last holder Done triggers sleep, orphan log, grace wait, close, dispose log.
Optional sleeper/waker/closer discovered via type switch (Yaegi-safe, not comma-ok).
DefaultGrace = 10s. MsgPut/Bind/Orphan/Reclaim/Dispose slog keys.
Imports: context, fmt, log/slog, sync, time only.
