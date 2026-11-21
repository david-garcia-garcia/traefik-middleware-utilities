// Package traefikemulator constructs one Traefik configuration generation at a time.
// Apply cancels the previous generation, then calls the plugin constructor for every
// route on one new context. Serve calls the current generation's handler by route name.
package traefikemulator

import (
	"context"
	"fmt"
	"net/http"
)

// Constructor is the plugin New a test passes in. config is whatever that New takes.
type Constructor func(ctx context.Context, next http.Handler, config any, middlewareName string) (http.Handler, error)

// Route is one router in a generation. Name is the Serve address. MiddlewareName is
// the constructor's name argument; empty uses Name.
type Route struct {
	Name           string
	MiddlewareName string
	Next           http.Handler
	Config         any
}

// Emulator is the process-wide stand-in for RouterFactory.CreateRouters.
type Emulator struct {
	newMiddleware Constructor
	cancel        context.CancelFunc
	routes        map[string]http.Handler
}

// New returns an emulator that constructs routes with newMiddleware.
func New(newMiddleware Constructor) *Emulator {
	if newMiddleware == nil {
		panic("traefikemulator: New requires a constructor")
	}
	return &Emulator{newMiddleware: newMiddleware}
}

// Apply cancels the previous generation, then constructs routes in order on one context.
// A constructor error omits that route and leaves the generation context live.
// The returned map is nil when every route was constructed.
func (e *Emulator) Apply(routes []Route) map[string]error {
	if e.cancel != nil {
		e.cancel()
	}
	generation, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	constructed := make(map[string]http.Handler, len(routes))
	seen := make(map[string]struct{}, len(routes))
	var failed map[string]error
	for _, route := range routes {
		if _, already := seen[route.Name]; already {
			failed = recordFailure(failed, route.Name, fmt.Errorf("traefikemulator: duplicate route %q", route.Name))
			continue
		}
		seen[route.Name] = struct{}{}
		middlewareName := route.MiddlewareName
		if middlewareName == "" {
			middlewareName = route.Name
		}
		handler, err := e.newMiddleware(generation, route.Next, route.Config, middlewareName)
		if err != nil {
			failed = recordFailure(failed, route.Name, err)
			continue
		}
		constructed[route.Name] = handler
	}
	e.routes = constructed
	return failed
}

// Stop cancels the current generation and drops its handlers.
func (e *Emulator) Stop() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.routes = nil
}

// Handler is the current generation's middleware for routeName.
func (e *Emulator) Handler(routeName string) (http.Handler, bool) {
	handler, ok := e.routes[routeName]
	return handler, ok
}

// Serve calls the current generation's handler. False means that route was not constructed.
func (e *Emulator) Serve(routeName string, w http.ResponseWriter, req *http.Request) bool {
	handler, ok := e.routes[routeName]
	if !ok {
		return false
	}
	handler.ServeHTTP(w, req)
	return true
}

func recordFailure(failed map[string]error, routeName string, err error) map[string]error {
	if failed == nil {
		failed = map[string]error{}
	}
	failed[routeName] = err
	return failed
}
