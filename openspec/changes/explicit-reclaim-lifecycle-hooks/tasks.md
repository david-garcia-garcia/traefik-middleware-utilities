## 1. Open signature and slot hooks

- [ ] 1.1 Add `Hooks` in `reclaim/` (`Sleep`, `Wake`, `Close func()`). Change `Table.Open` and package `Open` to `Open(ctx, key, logger, create, hooks)`. Store `hooks` on the slot in `put`. Bind and reclaim keep the stored funcs.
- [ ] 1.2 Delete `sleeper`, `waker`, `closer` and type-switch `sleepValue` / `wakeValue` / `closeValue`. Drive sleep, wake, and close from the stored funcs (nil skips). `table.go` stays stdlib-only.
- [ ] 1.3 Update every compiled `Open` call in `reclaim/table_test.go` to pass `Hooks` (method values or a test-only helper). Empty `Hooks{}` where no lifecycle is asserted. Add a test that a later `Open`'s hooks argument is ignored.

## 2. Yaegi interpreter tests

- [ ] 2.1 Add test-only `github.com/traefik/yaegi v0.16.1` on the module `go.mod`. No Yaegi import in non-test library files. No Yaegi in `e2e/reclaimprobe/go.mod`.
- [ ] 2.2 Add `reclaim` Yaegi tests (`yaegi_test.go` or `*_yaegi_test.go`): GOPATH interp, `stdlib.Symbols`, no unsafe. Prove type-switch on create `any` does not match, and `Open` `Hooks` run sleep/wake/close.

## 3. Probe host and Pester

- [ ] 3.1 Change `e2e/reclaimprobe` `New` to pass log-only `Hooks` (stable probe slog lines) with the stored value created inside `create`.
- [ ] 3.2 Extend `scripts/integration-tests.Tests.ps1`: keep put/bind and shared identity. Stop **both** whoami services, then start them within `DefaultGrace`; assert `reclaim_orphan`, `reclaim_reclaim`, and sleep/wake probe lines. Leave them down past grace; assert `reclaim_dispose` and the close probe line.

## 4. Docs and debt

- [ ] 4.1 Update `knowledge/devdocs/std_go_reclaim.md` (Open, Sleep, Wake, snippet close-over, drop Yaegi-inert gotcha). Delete `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md`.
- [ ] 4.2 Run `go test ./reclaim/...` then Pester e2e (`Test-Integration.ps1`) on this host until passing. `openspec validate --change explicit-reclaim-lifecycle-hooks --strict`.
