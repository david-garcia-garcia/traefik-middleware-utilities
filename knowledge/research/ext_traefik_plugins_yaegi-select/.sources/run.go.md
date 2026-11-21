---
url: https://github.com/traefik/yaegi/blob/v0.16.1/interp/run.go
title: Yaegi v0.16.1 interp/run.go select, recv and send
fetched: 2026-11-21
authority: source
ref: github.com/traefik/yaegi@v0.16.1:interp/run.go
---

Read from the local module cache (`github.com/traefik/yaegi@v0.16.1/interp/run.go`).

`func _select(n *node)` (line 3749), at node setup, outside `n.exec`:

```go
nbClause := len(n.child)
...
cases := make([]reflect.SelectCase, nbClause+1)
```

Clause directions (`cases[i].Dir`) are set at setup. Then:

```go
n.exec = func(f *frame) bltn {
    f.mutex.RLock()
    cases[nbClause] = f.done
    f.mutex.RUnlock()

    for i := range cases[:nbClause] {
        switch cases[i].Dir {
        case reflect.SelectRecv:
            cases[i].Chan = chanValues[i](f)
        case reflect.SelectSend:
            cases[i].Chan = chanValues[i](f)
            cases[i].Send = assignedValues[i](f)
        ...
        }
    }
    j, v, s := reflect.Select(cases)
    if j == nbClause {
        return nil
    }
    if cases[j].Dir == reflect.SelectRecv && assignedValues[j] != nil {
        assignedValues[j](f).Set(v)
        ...
    }
    return clause[j]
}
```

No lock covers the `cases` writes and the `reflect.Select` call. The `RLock` covers only the read of `f.done`.

`func recv` (3517), `func recv2` (3593), `func send` (3685): when `n.interp.cancelChan` is false they call `value(f).Recv()` / `value0(f).Send(...)` directly. When true they try `TryRecv` / `TrySend`, then call `reflect.Select([]reflect.SelectCase{done, {Dir: ..., Chan: ch}})` with a slice literal built inside `n.exec` on each run. No shared slice.
