## Purpose

Gives Traefik plugin tests a stand-in for one RouterFactory configuration generation at a time: construct middleware handlers on a shared context, cancel the prior generation on reload, and invoke handlers by route name without running Traefik.

## ADDED Requirements

### Requirement: Constructor is required at creation
`New` SHALL panic when the plugin constructor argument is nil. Otherwise it SHALL return an emulator bound to that constructor.

#### Scenario: Nil constructor panics
- **WHEN** `New` is called with a nil constructor
- **THEN** the call panics with a message that a constructor is required

### Requirement: Apply replaces the previous generation
Each `Apply` SHALL cancel the context passed to the previous generation before invoking the constructor for the new generation. Routes in one `Apply` SHALL share one cancellable context. A constructor error for one route SHALL omit that route from the active map and MUST NOT cancel the generation context for sibling routes that succeeded.

#### Scenario: Previous generation is cancelled before the next New runs
- **WHEN** `Apply` runs twice on the same emulator
- **THEN** the first generation's context is cancelled before the constructor runs for the second generation

#### Scenario: Routes in one Apply share one context
- **WHEN** `Apply` lists more than one route
- **THEN** every successful constructor call for that list receives the same context value

#### Scenario: Failed constructor leaves siblings live
- **WHEN** `Apply` lists routes where one constructor returns an error
- **THEN** that route name is absent from the active map
- **AND** the generation context for successful routes is not cancelled
- **AND** `Apply` returns an error entry for the failed route name

#### Scenario: Duplicate route name in one Apply is rejected
- **WHEN** `Apply` lists the same route name twice in one call
- **THEN** the second occurrence is not constructed
- **AND** `Apply` returns an error entry for that route name
- **AND** the first occurrence remains constructed when its constructor succeeded

#### Scenario: Omitted routes from a later Apply are not in the map
- **WHEN** a later `Apply` lists only a subset of names from an earlier generation
- **THEN** only routes named in the latest successful `Apply` are reachable

#### Scenario: Shared middleware name still constructs each route
- **WHEN** two routes use the same middleware name in one `Apply`
- **THEN** the constructor is invoked once per route
- **AND** cancelling that generation cancels every route context from that `Apply`

### Requirement: Stop ends the current generation
`Stop` SHALL cancel the current generation context if one exists, clear the cancel function, and drop all constructed handlers. After `Stop`, lookup and serve for any route name SHALL behave as if no generation is active.

#### Scenario: Stop cancels route contexts
- **WHEN** `Apply` has succeeded for one or more routes
- **AND** `Stop` is called
- **THEN** every context from that generation reports cancelled

#### Scenario: Stop clears handlers
- **WHEN** `Stop` is called after a successful `Apply`
- **THEN** `Handler` and `Serve` for a formerly constructed route name return not found

### Requirement: Handler and Serve address the current generation only
`Handler(routeName)` SHALL return the handler for `routeName` when that route was constructed in the latest successful `Apply`, otherwise false. `Serve(routeName, w, req)` SHALL invoke that handler with the supplied response writer and request when present, otherwise return false without writing. The emulator MUST NOT modify the request (no Host, client IP, or header rewriting).

#### Scenario: Serve hits the latest generation
- **WHEN** two successive `Apply` calls construct the same route name with different handlers
- **THEN** `Serve` uses the handler from the second `Apply`

#### Scenario: Serve forwards the test request unchanged
- **WHEN** `Serve` is called with a specific `*http.Request`
- **THEN** the constructed handler receives that same request value

#### Scenario: Missing route does not serve
- **WHEN** `Serve` or `Handler` is called for a route name not in the current map
- **THEN** `Serve` returns false
- **AND** `Handler` returns false

### Requirement: Package depends only on the Go standard library
The `traefikemulator` package source SHALL import only Go standard-library packages. It MUST NOT import Traefik, Yaegi, or other packages from this module.

#### Scenario: Stdlib-only imports
- **WHEN** production sources in `traefikemulator/` are listed for imports
- **THEN** every import path is a Go standard-library package
