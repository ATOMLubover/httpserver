package httpserver

import (
	"fmt"
)

// Route tree implemented with Trie.
type _RouteTree struct {
	roots []*_RouteNode // ref of root node of route tree
}

// Create a new route tree.
func _NewRouteTree() *_RouteTree {
	t := &_RouteTree{
		roots: make([]*_RouteNode, 5),
	}

	for i := range t.roots {
		t.roots[i] = _NewRouteNode()
	}

	return t
}

// Insert route into route tree.
func (t *_RouteTree) _Insert(method Method, pattern string, group *RouteGroup, handler HandlerFunc) {
	// parse pattern
	patternParts, err := _TransformPatternIntoParts(pattern)
	if err != nil {
		panic("invalid pattern: " + pattern)
	}

	// insert route node
	t.roots[method]._Insert(pattern, patternParts, group, handler)
}

// Try searching a matching route node, return nil if not found or uri is invalid.
func (t *_RouteTree) _Search(uri string, method Method, uriParams *map[string]string) *_RouteNode {
	uriParts, err := _TransformUriIntoParts(uri)
	if err != nil {
		gLogger.Debug(fmt.Sprintf("Invalid uri access: %s, method: %s", uri, method))
		return nil
	}

	// here may return nil if not found
	node := t.roots[method]._Find(uriParts, 0, uriParams)
	return node
}
