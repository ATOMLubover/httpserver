// router
package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
)

// route tree node
// implemented with Trie tree
type _RouteNode struct {
	pattern    string // entire route pattern
	part       string // current route part
	isWildcard bool   // is it a wildcard part
	isLeaf     bool   // is it a leaf node

	children []*_RouteNode // children nodes

	handler HandlerFunc // corresponding handler
}

// insert route node
func (n *_RouteNode) _Insert(fullPattern string, patternParts []string, handler HandlerFunc) {
	// n is parent actually, so just end the recursion and modify n
	if len(patternParts) == 0 {
		if n.handler != nil {
			slog.Warn(fmt.Sprintf("Route '%s' already exists, overwriting...",
				fullPattern))
		}

		n.pattern = fullPattern
		n.isLeaf = true
		n.handler = handler

		slog.Info("Inserted route: " + fullPattern)

		return
	}

	currentPart := patternParts[0]
	remainingParts := patternParts[1:]

	// try match existing perfect matching nodes
	// perfect match is priority
	for _, child := range n.children {
		// if find matching node, recurse on it
		if child.part == currentPart {
			// NOTICE that asterisk matching also needs an end
			if currentPart == "*" && len(remainingParts) > 0 {
				panic("wildcard can only be the last part")
			}

			child._Insert(fullPattern, remainingParts, handler)
			return
		}
	}

	// only when perfect match not found, fallthrough here
	// preprocess if currentPart is wildcard
	// because wildcard need follow extra rules
	if currentPart[0] == '*' {
		err := _ValidateAsteriskWildcard(currentPart, len(remainingParts))
		if err != nil {
			panic("invalid asterisk part '" + err.Error() + "' in pattern: " + fullPattern)
		}

		for _, child := range n.children {
			if child.isWildcard &&
				// rule: no two wildcards in one level
				child.part != currentPart {
				panic("conflicting wildcard routes: '" + child.part + "' vs '" + currentPart + "'")
			}
		}
	}
	if currentPart[0] == ':' {
		err := _ValidateColonWildcard(currentPart)
		if err != nil {
			panic("invalid colon part '" + err.Error() + "' in pattern: " + fullPattern)
		}

		for _, child := range n.children {
			if child.isWildcard &&
				// rule: no two wildcards in one level
				child.part != currentPart {
				panic("conflicting wildcard routes: '" + child.part + "' vs '" + currentPart + "'")
			}
		}
	}

	// new exact currentPath and verified wildcard currentPath will come here
	// create corresponding node
	newNode := &_RouteNode{
		part:       currentPart,
		isWildcard: currentPart[0] == ':' || currentPart == "*",
	}

	// afterall, do not allow asterisk wildcard as a parent of another wildcard
	if currentPart == "*" {
		if len(remainingParts) > 0 {
			panic("wildcard '*' must be the last part")
		}
		newNode.isLeaf = true
		newNode.pattern = fullPattern
		newNode.handler = handler
	}

	// grow the route tree
	n.children = append(n.children, newNode)
	// recurse to insert the remaining parts
	if currentPart != "*" {
		newNode._Insert(fullPattern, remainingParts, handler)
	}
}

// // use recursion to find previously inserted node to handle request
// // with node itself and params resolved
// func (n *_RouteNode) _Find(parts []string, height int, params *map[string]string) *_RouteNode {
// 	// end recursion when reaching target height
// 	if len(parts) == height {
// 		// if it is leaf node, return it
// 		if n.isLeaf {
// 			return n
// 		}
// 		return nil
// 	}

// 	// get current part
// 	part := parts[height]
// 	children := n._MatchChildren(part)

// 	for _, child := range children {
// 		// save previous param state for backtracking
// 		var oldValue string
// 		var hasOld bool

// 		// save param value
// 		if child.part[0] == ':' {
// 			paramKey := child.part[1:]
// 			oldValue, hasOld = (*params)[paramKey]
// 			(*params)[paramKey] = part
// 		}

// 		// then recurse to find
// 		result := child._Find(parts, height+1, params)
// 		if result != nil {
// 			return result
// 		}

// 		// backtrace after dfs
// 		if child.part[0] == ':' {
// 			paramKey := child.part[1:]
// 			if hasOld {
// 				(*params)[paramKey] = oldValue
// 			} else {
// 				delete(*params, paramKey)
// 			}
// 		}

// 		// specfically process asterisk
// 		if child.part[0] == '*' {
// 			if len(parts) >= height {
// 				paramKey := strings.TrimPrefix(child.part, "*")
// 				if paramKey == "" {
// 					paramKey = "_"
// 				}
// 				(*params)[paramKey] = strings.Join(parts[height:], "/")
// 				return child
// 			}
// 		}
// 	}

// 	// return nil when no mathcing node
// 	return nil
// }

func (n *_RouteNode) _Find(patternParts []string, height int, params *map[string]string) *_RouteNode {
	// end recursion when matching all parts
	if height == len(patternParts) {
		if n.isLeaf {
			slog.Debug(fmt.Sprintf("Leaf node matched: %v, params: %v",
				n.pattern, *params))
			return n
		}
		return nil
	}

	currentPart := patternParts[height]

	// firstly try match exact node
	for _, child := range n.children {
		if child.isWildcard || child.part != currentPart {
			continue
		}

		found := child._Find(patternParts, height+1, params)
		if found != nil {
			return found
		}
	}

	// if failed, try match wildcard node
	for _, child := range n.children {
		if !child.isWildcard {
			continue
		}

		switch child.part[0] {
		case ':':
			// refuse empty string
			if currentPart == "" {
				slog.Debug("colon meets empty string")
				continue
			}

			// before recursion, save current param value
			paramKey := child.part[1:]
			if params != nil {
				(*params)[paramKey] = currentPart
			}

			found := child._Find(patternParts, height+1, params)
			if found != nil {
				return found
			}

			// if find failed, restore current param value
			if params != nil {
				delete(*params, paramKey)
			}

		case '*':
			// asterisk param key name is default as "*"
			paramKey := "*"
			if len(child.part[1:]) > 0 {
				paramKey = child.part[1:]
			}

			if params != nil {
				// asterisk will match the rest part of the uri
				(*params)[paramKey] = strings.Join(patternParts[height:], "/")
			}

			slog.Debug(fmt.Sprintf("Asterisk node matched: %v, params: %v",
				child.pattern, *params))
			return child
		}
	}

	return nil
}

// seek through children of n to find whether there is a child node
// with the given pattern part when _Insert
func (n *_RouteNode) _FindChild(part string) *_RouteNode {
	for _, child := range n.children {
		if child.part == part {
			return child
		}
	}
	return nil
}

// seek through children of n to find children nodes
// matching the given part when _Find
// pay attention to the order of nodes(exact > ":" > "*")
func (n *_RouteNode) _MatchChildren(part string) []*_RouteNode {
	nodes := make([]*_RouteNode, 0, 3)

	if node := n._FindChild(part); node != nil {
		nodes = append(nodes, node)
	}

	for _, child := range n.children {
		if child.isWildcard && child.part[0] == ':' {
			nodes = append(nodes, child)
			break // take firstly matched option
		}
	}

	for _, child := range n.children {
		if child.isWildcard && child.part[0] == '*' {
			nodes = append(nodes, child)
			break // take firstly matched option
		}
	}

	return nodes
}

// router controlls the all routes
// and provides interfaces to register routes and handle requests
type _Router struct {
	root     *_RouteNode            // root node of route tree
	handlers map[string]HandlerFunc // references to handler
}

// create a new router
func _NewRouter() *_Router {
	return &_Router{
		root: &_RouteNode{
			pattern:    "",
			part:       "",
			isWildcard: false,
			children:   make([]*_RouteNode, 0),
			handler:    nil,
		},
	}
}

// transform a URI into pattern parts when setting route
func _ParseParts(pattern string) ([]string, error) {
	// trim slashes
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return []string{}, nil // return empty slice when at root
	}

	parts := strings.Split(trimmed, "/")
	result := make([]string, 0, len(parts))

	for i, part := range parts {
		// continue when part is empty
		if part == "" {
			continue
		}

		// check wildcard rules
		switch {
		case strings.HasPrefix(part, ":"):
			if err := _ValidateColonWildcard(part); err != nil {
				return nil, fmt.Errorf("%w in: %s", err, pattern)
			}

		case strings.HasPrefix(part, "*"):
			if err := _ValidateAsteriskWildcard(part, len(parts)-i-1); err != nil {
				return nil, fmt.Errorf("%w in: %s", err, pattern)
			}
			result = append(result, part)
			return result, nil // return immidiately when reaching asterisk
		}

		result = append(result, part)
	}
	return result, nil
}

// validate colon wildcard rules
func _ValidateColonWildcard(part string) error {
	// format should be like ":name"
	if len(part) == 1 {
		return errors.New("colon wildcard must be named")
	}
	// colon should not be repeated
	if strings.Count(part, ":") > 1 {
		return errors.New("too many ':' in part: " + part)
	}
	// wildcard naming should be valid
	if !_CheckColonNameValid(strings.TrimPrefix(part, ":")) {
		return errors.New("invalid wildcard name: " + part)
	}

	return nil
}

// validate asterisk wildcard rules
func _ValidateAsteriskWildcard(part string, remainingPartsNum int) error {
	// make sure asterisk is last part
	if remainingPartsNum >= 1 {
		return errors.New("asterisk must be last part")
	}

	// validate naming
	name := strings.TrimPrefix(part, "*")
	if !_CheckAsteriskNameValid(name) {
		return errors.New("invalid asterisk name: " + name)
	}

	return nil
}

// check colon naming validation
func _CheckColonNameValid(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

// check asterisk naming validation
func _CheckAsteriskNameValid(name string) bool {
	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}

	// return true after checking
	return true
}

func (r *_Router) _AddRoute(method Method, pattern string, handler HandlerFunc) {
	if !CheckValidUri(pattern) {
		slog.Warn("URI pattern format: " + pattern)
		return
	}

	// record handlers into router
	key := method.String() + "-" + pattern
	r.handlers[key] = handler

	// splice route pattern into parts in order to insert into route tree
	parts, err := _ParseParts(pattern)
	if err != nil || len(parts) == 0 {
		slog.Warn(fmt.Sprintf("Invalid route pattern: %s, error: %s", pattern, err.Error()))
		return
	}

	// though root should not be nil after NewRouter function
	// here make sure that root cannot be nil
	if r.root == nil {
		// root should be a virtual node
		r.root = &_RouteNode{
			pattern:    "",
			part:       "",
			isWildcard: false,
			children:   make([]*_RouteNode, 0),
			handler:    nil,
		}
	}
	// insert into route tree
	r.root._Insert(pattern, parts, handler)
}
