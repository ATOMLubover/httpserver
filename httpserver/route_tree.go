package httpserver

import (
	"fmt"
	"log/slog"
)

// route tree implemented with Trie
type _RouteTree struct {
	root *_RouteNode // ref of root node of route tree
}

// Create a new route tree
func _NewRouteTree() *_RouteTree {
	return &_RouteTree{
		root: _NewRouteNode(),
	}
}

// insert route node
func (t *_RouteTree) _Insert(method Method, pattern string, handler HandlerFunc) {
	// parse pattern
	patternParts, err := _TransformPatternIntoParts(pattern)
	if err != nil {
		panic("invalid pattern: " + pattern)
	}

	// insert route node
	t.root._Insert(pattern, patternParts, method, handler)
}

// try searching a matching route node, return nil if not found or uri is invalid
func (t *_RouteTree) _Search(uri string, method Method) *_RouteNode {
	uriParts, err := _TransformUriIntoParts(uri)
	if err != nil {
		slog.Debug(fmt.Sprintf("invalid uri access: %s, method: %s", uri, method))
		return nil
	}

	// here may return nil if not found
	return t.root._Find(uriParts, 0, method, nil)
}
