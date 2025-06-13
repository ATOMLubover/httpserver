package httpserver

import (
	"fmt"
	"strings"
)

// Route tree node.
type _RouteNode struct {
	group *RouteGroup // the route group the node belongs to

	pattern    string // entire route pattern
	part       string // current route part
	isWildcard bool   // is it a wildcard part
	isLeaf     bool   // is it a leaf node

	children []*_RouteNode // children nodes

	handler HandlerFunc // corresponding handlers
}

// Create a new route node
func _NewRouteNode() *_RouteNode {
	return &_RouteNode{
		group: nil,

		pattern:    "",
		part:       "",
		isWildcard: false,
		isLeaf:     false,

		children: make([]*_RouteNode, 0),

		handler: nil,
	}
}

// Insert node by recursion
func (n *_RouteNode) _Insert(fullPattern string, patternParts []string, group *RouteGroup, handler HandlerFunc) {
	// n is parent actually, so just end the recursion and modify n
	if len(patternParts) == 0 {
		if n.handler != nil {
			gLogger.Warn(fmt.Sprintf("Route '%s' already exists, overwriting...", fullPattern))
		}

		// Set the route group of the node inserted.
		n.group = group

		n.pattern = fullPattern
		n.isLeaf = true
		n.handler = handler

		gLogger.Debug("Inserted route: " + fullPattern)

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

			child._Insert(fullPattern, remainingParts, group, handler)
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
	newNode := _NewRouteNode()
	newNode.part = currentPart
	newNode.isWildcard = currentPart[0] == ':' || currentPart == "*"

	// afterall, do not allow asterisk wildcard as a parent of another wildcard
	if strings.HasPrefix(currentPart, "*") {
		if len(remainingParts) > 0 {
			panic("wildcard '*' must be the last part")
		}

		newNode.group = group

		newNode.isLeaf = true
		newNode.isWildcard = true
		newNode.pattern = fullPattern
		newNode.handler = handler

		n.children = append(n.children, newNode)

		return
	}

	// grow the route tree
	n.children = append(n.children, newNode)
	// recurse to insert the remaining parts
	newNode._Insert(fullPattern, remainingParts, group, handler)
}

// Find node by recursion
func (n *_RouteNode) _Find(uriParts []string, height int, params *map[string]string) *_RouteNode {
	// end recursion when matching all parts
	if height == len(uriParts) {
		if n.isLeaf {
			gLogger.Debug(fmt.Sprintf("Leaf node matched: %v, params: %v", n.pattern, *params))
			return n
		}
		gLogger.Debug(fmt.Sprintf("No leaf node matched with: %v", uriParts))
		return nil
	}

	currentPart := uriParts[height]

	// firstly try match exact node
	for _, child := range n.children {
		if child.isWildcard || child.part != currentPart {
			continue
		}

		found := child._Find(uriParts, height+1, params)
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
				gLogger.Debug("colon meets empty string")
				continue
			}

			// before recursion, save current param value
			paramKey := child.part[1:]
			if params != nil {
				(*params)[paramKey] = currentPart
			}

			found := child._Find(uriParts, height+1, params)
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
				(*params)[paramKey] = strings.Join(uriParts[height:], "/")
			}

			gLogger.Debug(fmt.Sprintf("Asterisk node matched: %v, params: %v",
				child.pattern, *params))
			return child
		}
	}

	return nil
}
