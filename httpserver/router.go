// router
package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
)

// router controlls the all routes
// and provides interfaces to register routes and handle requests
type _Router struct {
	RouteGroup // inherit RouteGroup

	handlers map[string]HandlerFunc // references to handler
}

// implement http.Handler interface
func (r *_Router) _ServeHttpImpl(w http.ResponseWriter, req *http.Request) {

}

// create a new router
func _NewRouter() *_Router {
	return &_Router{
		RouteGroup: RouteGroup{
			routeTree:   _NewRouteTree(),
			parent:      nil,
			children:    make([]*RouteGroup, 0),
			prefix:      "/",
			middlewares: make([]MiddlewareFunc, 0),
		},
		handlers: make(map[string]HandlerFunc),
	}
}

// Add a route to router
func (r *_Router) AddRoute(method Method, pattern string, handler HandlerFunc) {
	if !_CheckPatternCharactors(pattern) {
		panic("invalid URI pattern format: " + pattern)
	}

	// process root route specially
	if pattern == "/" {
		r.routeTree.root.handlers[method] = handler
		r.routeTree.root.pattern = pattern
		r.routeTree.root.isLeaf = true

		key := method.String() + "-" + pattern
		r.handlers[key] = handler

		slog.Info(fmt.Sprintf("Added root in router: %s, method: %s", pattern, method))

		return
	}

	// splice route pattern into parts in order to insert into route tree
	parts, err := _TransformPatternIntoParts(pattern)
	if err != nil || len(parts) == 0 {
		panic(err.Error())
	}

	// insert into route tree
	r.routeTree._Insert(method, pattern, handler)

	// record handlers into router
	key := method.String() + "-" + pattern
	r.handlers[key] = handler

	slog.Info(fmt.Sprintf("Route added in router: %s [%s]", pattern, method))
}

// Add a new child route group into this route group.
// Return the child route group newly created.
func (r *_Router) AddGroup(prefix string) *RouteGroup {
	// remove the last '/'
	parentPrefix := r.prefix[:len(r.prefix)-1]
	// create child route group
	child := _NewRouteGroup(&r.RouteGroup, parentPrefix+prefix)

	// add child route group to children
	r.children = append(r.children, child)

	return child
}

// Use a new middleware into router
func (r *_Router) UseMiddleware(middleware MiddlewareFunc) {
	r.middlewares = append(r.middlewares, middleware)
}

// Handle request.
func (r *_Router) Handle(w http.ResponseWriter, req *http.Request) {
	
}
