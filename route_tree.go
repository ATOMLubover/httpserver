package httpserver

import (
	"fmt"
)

// Route tree implemented with Trie.
type _RouteTree struct {
	root *_RouteNode // ref of root node of route tree
}

// Create a new route tree.
func _NewRouteTree() *_RouteTree {
	return &_RouteTree{
		root: _NewRouteNode(),
	}
}

// Insert route into route tree.
func (t *_RouteTree) _Insert(method Method, pattern string, group *RouteGroup, handler HandlerFunc) {
	// parse pattern
	patternParts, err := _TransformPatternIntoParts(pattern)
	if err != nil {
		panic("invalid pattern: " + pattern)
	}

	// insert route node
	t.root._Insert(pattern, patternParts, group, method, handler)
}

// Try searching a matching route node, return nil if not found or uri is invalid.
func (t *_RouteTree) _Search(uri string, method Method) (*_RouteNode, map[string]string) {
	uriParts, err := _TransformUriIntoParts(uri)
	if err != nil {
		gLogger.Debug(fmt.Sprintf("Invalid uri access: %s, method: %s", uri, method))
		return nil, nil
	}

	params := make(map[string]string)
	// here may return nil if not found
	node := t.root._Find(uriParts, 0, method, &params)
	return node, params
}
