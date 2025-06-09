package httpserver

import (
	"strings"
)

// Route group
type RouteGroup struct {
	routeTree *_RouteTree // ref of route tree

	middlewareCache *_MiddlewareChainCache // ref of middleware cache

	parent   *RouteGroup   // parent group
	children []*RouteGroup // children group

	prefix      string           // shared prefix(should end with '/')
	middlewares []MiddlewareFunc // shared middlewares
}

// Create a new route group.
func _NewRouteGroup(middlewareCache *_MiddlewareChainCache, parent *RouteGroup, prefix string) *RouteGroup {
	// check the format of prefix
	{
		if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
			panic("prefix should start with '/' and end with '/': " + prefix)
		}

		if strings.Count(prefix, "*") != 0 {
			panic("prefix should not contain '*' wildcard: " + prefix)
		}

		if !_CheckPatternCharactors(prefix) {
			panic("pattern including invalid charactors: " + prefix)
		}

		prefixParts, err := _TransformPatternIntoParts(prefix)
		if err != nil {
			panic("error when transforming prefix" + err.Error())
		}
		if len(prefixParts) == 0 {
			panic("prefix has empty parts: " + prefix)
		}
	}

	return &RouteGroup{
		routeTree:       parent.routeTree,
		middlewareCache: middlewareCache,

		parent:   parent,
		children: make([]*RouteGroup, 0),

		prefix:      prefix,
		middlewares: make([]MiddlewareFunc, 0), // inherit middlewares of parent runtime
	}
}

// Add a new route into this route group.
// Pattern should start with '/'.
func (g *RouteGroup) AddRoute(method Method, pattern string, handler HandlerFunc) {
	// pattern should start with '/'
	if !strings.HasPrefix(pattern, "/") {
		panic("child pattern should start with '/': " + pattern)
	}

	// add prefix to pattern
	// remove the last '/' of prefix
	pattern = g.prefix[:len(g.prefix)-1] + pattern

	// insert route node
	g.routeTree._Insert(method, pattern, g, handler)
}

// Add a new child route group into this route group.
// Return the child route group newly created.
func (g *RouteGroup) AddGroup(prefix string) *RouteGroup {
	// remove the last '/'
	parentPrefix := g.prefix[:len(g.prefix)-1]
	// create child route group
	child := _NewRouteGroup(g.middlewareCache, g, parentPrefix+prefix)

	// add child route group to children
	g.children = append(g.children, child)

	return child
}

// UseMiddleware insert new middleware(s) into this route group.
// Order of this function decides the order of middleware being applied,
// UseMiddleware firstly will be applied firstly.
func (g *RouteGroup) UseMiddleware(middlewares ...MiddlewareFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
	// Update middleware cache immidiately.
	// This operation would cause a very long time pause.
	g.middlewareCache._InvalidateMiddlewareChain(g.prefix)
}

// Get entire middleware chain of current route group.
func (g *RouteGroup) _GetMiddlewareChain() []MiddlewareFunc {
	// try to get middleware chain from cache
	middlewares := g.middlewareCache._Get(g.prefix, func() []MiddlewareFunc {
		// create middleware chain
		middlewares := make([]MiddlewareFunc, 0)
		// inherit middlewares of parent runtime
		// Branch here because router has no parent.
		if g.parent != nil {
			middlewares = append(middlewares, g.parent._GetMiddlewareChain()...)
		}
		// append middlewares of current route group
		middlewares = append(middlewares, g.middlewares...)

		return middlewares
	})

	return middlewares
}
