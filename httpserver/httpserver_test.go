package httpserver

import (
	"fmt"
	"strings"
	"testing"
)

func TestRouteInsert(t *testing.T) {
	root := &_RouteNode{part: "ROOT"}

	// 测试用例
	tests := []struct {
		pattern    string
		handler    HandlerFunc
		shouldFail bool
	}{
		// 正常路由
		{pattern: "/user/profile", handler: func(c Context) {}},
		// 参数路由
		{pattern: "/user/:id", handler: func(c Context) {}},
		{pattern: "/user/:id/profile", handler: func(c Context) {}},
		// 通配符路由
		{pattern: "/static/*", handler: func(c Context) {}},
		{pattern: "/static/files/*", handler: func(c Context) {}},
		// 冲突测试
		{pattern: "/user/:name", handler: func(c Context) {}, shouldFail: true},       // 与:id冲突
		{pattern: "/static/images", handler: func(c Context) {}},                      // 静态子节点
		{pattern: "/static/*/invalid", handler: func(c Context) {}, shouldFail: true}, // *后不能有路径
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldFail {
						t.Errorf("Unexpected panic: %v", r)
					}
				} else if tt.shouldFail {
					t.Error("Expected panic but none occurred")
				}
			}()

			parts := strings.Split(tt.pattern, "/")[1:]
			root._Insert(tt.pattern, parts, tt.handler)
		})
	}

	// 打印所有注册的路由
	root.printAllRoutes()

	// 验证路由是否插入正确
	assertNode(t, root, "static", "static", false)
	assertNode(t, root, "user", "user", false)

	// 验证通配符路由
	starNode := findChild(root, "static").children[0]
	if starNode.part != "*" || !starNode.isLeaf || starNode.pattern != "/static/*" {
		t.Errorf("Wildcard node incorrect: %+v", starNode)
	}

	// 验证参数路由
	paramNode := findChild(findChild(root, "user"), ":id")
	if paramNode.part != ":id" || !paramNode.isWildcard || !paramNode.children[0].isLeaf {
		t.Errorf("Param node incorrect: %+v", paramNode)
	}
}

// 辅助函数
func findChild(parent *_RouteNode, part string) *_RouteNode {
	for _, child := range parent.children {
		if child.part == part {
			return child
		}
	}
	return nil
}

func assertNode(t *testing.T, parent *_RouteNode, part string, expectPart string, expectLeaf bool) {
	node := findChild(parent, part)
	if node == nil {
		t.Fatalf("Node not found: %s", part)
	}
	if node.part != expectPart {
		t.Errorf("Expected part %s, got %s", expectPart, node.part)
	}
	if node.isLeaf != expectLeaf {
		t.Errorf("Expected isLeaf %v, got %v", expectLeaf, node.isLeaf)
	}
}

// helper to test: print the route tree
func (n *_RouteNode) printTree(level int) {
	indent := strings.Repeat("  ", level)
	wildcard := ""
	if n.isWildcard {
		wildcard = " [wildcard]"
	}
	leaf := ""
	if n.isLeaf {
		leaf = fmt.Sprintf(" -> %s", n.pattern)
	}
	fmt.Printf("%s- %s%s%s\n", indent, n.part, wildcard, leaf)

	for _, child := range n.children {
		child.printTree(level + 1)
	}
}

func (n *_RouteNode) printAllRoutes() {
	fmt.Println("\nRegistered Routes:")
	n.printTree(0)
}
