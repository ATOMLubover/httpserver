package httpserver

// import (
// 	"fmt"
// 	"log/slog"
// 	"os"
// 	"strings"
// 	"testing"
// )

// // helper to test: print the route tree
// func (n *_RouteNode) printTree(level int) {
// 	indent := strings.Repeat("  ", level)
// 	wildcard := ""
// 	if n.isWildcard {
// 		wildcard = " [wildcard]"
// 	}
// 	leaf := ""
// 	if n.isLeaf {
// 		leaf = fmt.Sprintf(" -> %s", n.pattern)
// 	}
// 	fmt.Printf("%s- %s%s%s\n", indent, n.part, wildcard, leaf)

// 	for _, child := range n.children {
// 		child.printTree(level + 1)
// 	}
// }

// func (n *_RouteNode) printAllRoutes() {
// 	fmt.Println("\nRegistered Routes:")
// 	n.printTree(0)
// }

// func TestRouteInsert(t *testing.T) {
// 	root := &_RouteNode{part: "ROOT"}

// 	// 测试用例
// 	tests := []struct {
// 		pattern    string
// 		handler    HandlerFunc
// 		shouldFail bool
// 	}{
// 		// 正常路由
// 		{pattern: "/user/profile", handler: func(c Context) {}},
// 		// 参数路由
// 		{pattern: "/user/:id", handler: func(c Context) {}},
// 		{pattern: "/user/:id/profile", handler: func(c Context) {}},
// 		// 通配符路由
// 		{pattern: "/static/*", handler: func(c Context) {}},
// 		{pattern: "/static/files/*", handler: func(c Context) {}},
// 		// 冲突测试
// 		{pattern: "/user/:name", handler: func(c Context) {}, shouldFail: true},       // 与:id冲突
// 		{pattern: "/static/images", handler: func(c Context) {}},                      // 静态子节点
// 		{pattern: "/static/*/invalid", handler: func(c Context) {}, shouldFail: true}, // *后不能有路径
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.pattern, func(t *testing.T) {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					if !tt.shouldFail {
// 						t.Errorf("Unexpected panic: %v", r)
// 					}
// 				} else if tt.shouldFail {
// 					t.Error("Expected panic but none occurred")
// 				}
// 			}()

// 			parts := strings.Split(tt.pattern, "/")[1:]
// 			root._Insert(tt.pattern, parts, tt.handler)
// 		})
// 	}

// 	// 打印所有注册的路由
// 	root.printAllRoutes()

// 	// 验证路由是否插入正确
// 	assertNode(t, root, "static", "static", false)
// 	assertNode(t, root, "user", "user", false)

// 	// 验证通配符路由
// 	starNode := findChild(root, "static").children[0]
// 	if starNode.part != "*" || !starNode.isLeaf || starNode.pattern != "/static/*" {
// 		t.Errorf("Wildcard node incorrect: %+v", starNode)
// 	}

// 	// 验证参数路由
// 	paramNode := findChild(findChild(root, "user"), ":id")
// 	if paramNode.part != ":id" || !paramNode.isWildcard || !paramNode.children[0].isLeaf {
// 		t.Errorf("Param node incorrect: %+v", paramNode)
// 	}
// }

// // 辅助函数
// func findChild(parent *_RouteNode, part string) *_RouteNode {
// 	for _, child := range parent.children {
// 		if child.part == part {
// 			return child
// 		}
// 	}
// 	return nil
// }

// func assertNode(t *testing.T, parent *_RouteNode, part string, expectPart string, expectLeaf bool) {
// 	node := findChild(parent, part)
// 	if node == nil {
// 		t.Fatalf("Node not found: %s", part)
// 	}
// 	if node.part != expectPart {
// 		t.Errorf("Expected part %s, got %s", expectPart, node.part)
// 	}
// 	if node.isLeaf != expectLeaf {
// 		t.Errorf("Expected isLeaf %v, got %v", expectLeaf, node.isLeaf)
// 	}
// }

// func TestRouteFind(t *testing.T) {
// 	root := &_RouteNode{part: "ROOT"}

// 	// 注册测试路由
// 	routes := []struct {
// 		pattern string
// 		handler HandlerFunc
// 	}{
// 		{"/user/profile", func(c Context) {}},
// 		{"/user/:id", func(c Context) {}},
// 		{"/user/:id/profile", func(c Context) {}},
// 		{"/static/*", func(c Context) {}},
// 		{"/static/files/*", func(c Context) {}},
// 		{"/product/:category/:id", func(c Context) {}},
// 	}

// 	for _, r := range routes {
// 		parts := strings.Split(r.pattern, "/")[1:]
// 		root._Insert(r.pattern, parts, r.handler)
// 	}

// 	tests := []struct {
// 		path    string
// 		found   bool
// 		pattern string
// 		params  map[string]string
// 	}{
// 		{"/user/profile", true, "/user/profile", nil},
// 		{"/user/123", true, "/user/:id", map[string]string{"id": "123"}},
// 		{"/user/456/profile", true, "/user/:id/profile", map[string]string{"id": "456"}},
// 		{"/static/css/style.css", true, "/static/*", map[string]string{"*": "css/style.css"}},
// 		{"/static/files/images/logo.png", true, "/static/files/*", map[string]string{"*": "images/logo.png"}},
// 		{"/product/books/789", true, "/product/:category/:id", map[string]string{"category": "books", "id": "789"}},
// 		{"/user/", false, "", nil},
// 		{"/static", false, "", nil},
// 		{"/unknown", false, "", nil},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.path, func(t *testing.T) {
// 			params := make(map[string]string)
// 			parts := strings.Split(tt.path, "/")[1:]

// 			node := root._Find(parts, 0, &params)

// 			if tt.found {
// 				if node == nil {
// 					t.Fatal("Expected to find node but got nil")
// 				}
// 				if node.pattern != tt.pattern {
// 					t.Errorf("Expected pattern %s, got %s", tt.pattern, node.pattern)
// 				}
// 				if len(params) != len(tt.params) {
// 					t.Errorf("Expected %d params, got %d", len(tt.params), len(params))
// 				}
// 				for k, v := range tt.params {
// 					if params[k] != v {
// 						t.Errorf("Param %s: expected %s, got %s", k, v, params[k])
// 					}
// 				}
// 			} else {
// 				if node != nil {
// 					t.Errorf("Expected nil node, got %s", node.pattern)
// 				}
// 			}
// 		})
// 	}
// }

// func TestAddRoute(t *testing.T) {
// 	// 创建 HandlerOptions 并设置日志级别为 Debug
// 	handlerOpts := &slog.HandlerOptions{
// 		Level: slog.LevelDebug, // 设置日志级别为 Debug
// 	}

// 	// 创建 JSON 格式的 Handler
// 	handler := slog.NewTextHandler(os.Stdout, handlerOpts)

// 	// 创建新的 Logger
// 	logger := slog.New(handler)

// 	// 设置为全局默认 Logger
// 	slog.SetDefault(logger)

// 	// 创建新的路由器
// 	router := _NewRouter()

// 	// 定义测试用例
// 	tests := []struct {
// 		name        string
// 		method      Method
// 		pattern     string
// 		handler     HandlerFunc
// 		shouldPanic bool
// 	}{
// 		// 有效路由
// 		{
// 			name:    "Valid static route",
// 			method:  GET,
// 			pattern: "/user/profile",
// 			handler: func(c Context) {},
// 		},
// 		{
// 			name:    "Valid parameter route",
// 			method:  GET,
// 			pattern: "/user/:id",
// 			handler: func(c Context) {},
// 		},
// 		{
// 			name:    "Valid named asterisk route",
// 			method:  GET,
// 			pattern: "/static/*filepath",
// 			handler: func(c Context) {},
// 		},
// 		{
// 			name:    "Valid unnamed asterisk route",
// 			method:  GET,
// 			pattern: "/assets/*",
// 			handler: func(c Context) {},
// 		},
// 		{
// 			name:    "Root route",
// 			method:  GET,
// 			pattern: "/",
// 			handler: func(c Context) {},
// 		},

// 		// 无效路由
// 		{
// 			name:        "Invalid colon wildcard",
// 			method:      GET,
// 			pattern:     "/user/:",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 		{
// 			name:        "Invalid wildcard name",
// 			method:      GET,
// 			pattern:     "/user/:123id",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 		{
// 			name:        "Asterisk not at end",
// 			method:      GET,
// 			pattern:     "/static/*/files",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 		{
// 			name:        "Invalid character in static part",
// 			method:      GET,
// 			pattern:     "/user-name/profile",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 		{
// 			name:        "Path traversal attempt",
// 			method:      GET,
// 			pattern:     "/static/../secret",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 		{
// 			name:        "Empty pattern",
// 			method:      GET,
// 			pattern:     "",
// 			handler:     func(c Context) {},
// 			shouldPanic: true,
// 		},
// 	}

// 	// 运行测试用例
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			var panicErr interface{}
// 			func() {
// 				defer func() {
// 					if r := recover(); r != nil {
// 						panicErr = r
// 						t.Logf("Panic occurred: %v", r) // 打印 panic 错误
// 					}
// 				}()

// 				// 添加路由
// 				router._AddRoute(tt.method, tt.pattern, tt.handler)

// 				fmt.Println("Finished adding route:", tt.pattern)
// 			}()

// 			if tt.shouldPanic {
// 				if panicErr == nil {
// 					t.Error("Expected panic but none occurred")
// 				}
// 			} else {
// 				if panicErr != nil {
// 					t.Errorf("Unexpected panic: %v", panicErr)
// 				} else {
// 					// 验证有效路由是否被正确添加
// 					key := tt.method.String() + "-" + tt.pattern
// 					if _, exists := router.handlers[key]; !exists {
// 						t.Errorf("Handler not registered for key: %s", key)
// 					}

// 					// 验证Trie树中是否添加了路由
// 					// parts := strings.Split(strings.Trim(tt.pattern, "/"), "/")
// 					// if len(parts) == 0 {
// 					// 	parts = []string{}
// 					// }
// 					parts, err := _ParseParts(tt.pattern)
// 					if err != nil {
// 						t.Errorf("Error parsing pattern: %v", err)
// 					}

// 					params := make(map[string]string)
// 					// 查找路由节点
// 					fmt.Printf("Try seeking for pattern: %s, parts: %v\n", tt.pattern, parts)
// 					node := router.root._Find(parts, 0, &params)
// 					if node == nil {
// 						t.Errorf("Route not found in trie: %s", tt.pattern)
// 					} else if node.pattern != tt.pattern {
// 						t.Errorf("Pattern mismatch. Expected: %s, Got: %s", tt.pattern, node.pattern)
// 					}
// 				}
// 			}
// 		})
// 	}

// 	// 额外测试：重复添加相同路由
// 	t.Run("Duplicate route registration", func(t *testing.T) {
// 		pattern := "/duplicate/route"
// 		handler1 := func(c Context) {}
// 		handler2 := func(c Context) {}

// 		// 第一次添加
// 		router._AddRoute(GET, pattern, handler1)

// 		// 第二次添加 - 应该覆盖之前的处理函数
// 		router._AddRoute(GET, pattern, handler2)

// 		key := GET.String() + "-" + pattern
// 		if router.handlers[key] == nil {
// 			t.Error("Handler not registered")
// 		} else if fmt.Sprintf("%p", router.handlers[key]) != fmt.Sprintf("%p", handler2) {
// 			t.Error("Handler not updated to the latest one")
// 		}
// 	})

// 	// 打印所有路由用于调试
// 	fmt.Println("\nRegistered Routes after TestAddRoute:")
// 	router.root.printAllRoutes()
// }
