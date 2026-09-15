# Brief: upstream reclaim gets OpenWithHooks + OpenTyped

Change 1: Table.OpenWithHooks plus put threading, in reclaim/table.go
The reclaim package lives at reclaim/ in the repo root, not pkg/reclaim.
Today Table.Open takes the lifecycle hooks BESIDE create:
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error)
Because the hooks argument is evaluated before Open runs, any caller whose hooks must act on the value being created has to declare a variable first, assign it inside create, and have the hook funcs close over it. In the consumer repo that boilerplate was copy-pasted three times.
Add a variant where create returns the hooks together with the value:
func (t *Table) OpenWithHooks(ctx context.Context, key string, logger *slog.Logger, create func() (any, Hooks, error)) (any, error)
Mechanically: put changes its signature to take create func() (any, Hooks, error) and drops its hooks parameter, captures the returned hooks into a local, and passes that local to publishPut. Nothing in the state machine changes, because publishPut already received hooks as a plain value. OpenWithHooks holds the loop that Open used to hold, and Open becomes a thin backward-compatible wrapper that wraps its create in a closure returning the fixed hooks value.
Backward compatibility is a hard requirement. Open keeps its exact signature, semantics and ERROR PRECEDENCE: nil table first, then nil logger, then nil create. Since the wrapping closure is never nil, Open must check its own create for nil before delegating, and inside that branch still report nil table and nil logger first. Preserve that ordering exactly; a nil-create call must not start returning a different error than it does today.
A real benefit of doing this inside the package rather than as an external helper: EnforceCloseBeforeOpen is a bool, not a func, so it cannot be faked with an indirection. Threading the hooks that create returns straight to publishPut makes it work properly.

Change 2: generic helper OpenTyped, new file reclaim/opentyped.go
A package-level generic FUNCTION (not a method) that calls OpenWithHooks and returns the stored value already typed:
func OpenTyped[T any](ctx context.Context, t *Table, key string, logger *slog.Logger, create func() (any, Hooks, error)) (T, error)
On a type mismatch it returns the zero T and an error of the form `reclaim: open %q: want %T, got %T`.

Validation already done in consumer traefik-geoblock under Yaegi (document as context, do not re-run):
- Host: Traefik v3.7.11, Yaegi v0.16.1, plugin loaded through --experimental.localplugins.
- All three consumer call sites rewritten onto OpenTyped: *BIN, *MMDB, *geoblock.Plugin. All three loaded and ran.
- Yaegi constraint: generic instantiation must remain a CALL EXPRESSION in a package that can name T.

Obligations for later phases (record as desired, do not implement now):
- Real tests for OpenWithHooks (EnforceCloseBeforeOpen honoured; later Open binds without re-running create) and OpenTyped (typed return, singleton identity, type-mismatch error). Regression that Open nil-argument error precedence is unchanged. Yaegi harness case if the repo has one.
- Update openspec/ specs and knowledge/ docs where they describe the hooks contract of Open.
- Do NOT create a git tag or bump a version. Consumer traefik-geoblock will need a version bump plus go mod vendor after merge.

Ready-made artifacts exist at D:\tmp\reclaim-upstream-patch\ (table.go.diff, opentyped.go) — mention as context; do not apply them in prepare.
