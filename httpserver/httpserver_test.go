package httpserver

import (
	"fmt"
	"log/slog"
	"os"

	//"os"
	"strings"
	"testing"
)

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

func (n *_RouteNode) printTree(level int) {
	indent := strings.Repeat("  ", level)
	wildcard := ""
	if n.isWildcard {
		wildcard = " [wildcard]"
	}
	leaf := ""
	if n.isLeaf {
		methods := make([]string, 0)
		for method := range n.handlers {
			methods = append(methods, method.String())
		}
		leaf = fmt.Sprintf(" ------> %s %v", n.pattern, methods)
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

func TestRouteTreeInsert(t *testing.T) {
	// 创建 HandlerOptions 并设置日志级别为 Debug
	handlerOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // 设置日志级别为 Debug
	}

	// 创建 JSON 格式的 Handler
	handler := slog.NewTextHandler(os.Stdout, handlerOpts)

	// 创建新的 Logger
	logger := slog.New(handler)

	// 设置为全局默认 Logger
	slog.SetDefault(logger)

	// 创建路由树实例
	tree := _NewRouteTree()

	// 辅助函数：安全执行可能panic的操作
	testPanic := func(name string, f func()) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("%s: got expected panic: %v", name, r)
			}
		}()
		f()
	}

	// 1. 测试普通路由插入
	t.Run("Normal Routes", func(t *testing.T) {
		tree._Insert(GET, "/api/users", func(c *Context) {})
		tree._Insert(POST, "/api/users", func(c *Context) {})
		tree._Insert(GET, "/api/products", func(c *Context) {})
		// 无panic即成功
	})

	// 2. 测试通配路由
	t.Run("Wildcard Routes", func(t *testing.T) {
		// 冒号通配
		tree._Insert(GET, "/users/:id", func(c *Context) {})
		tree._Insert(GET, "/users/:id/profile", func(c *Context) {})

		// 星号通配
		tree._Insert(GET, "/static/*", func(c *Context) {})
		// 无panic即成功
	})

	// 3. 测试同一路由不同方法
	t.Run("Same Route Different Methods", func(t *testing.T) {
		tree._Insert(GET, "/dashboard", func(c *Context) {})
		tree._Insert(POST, "/dashboard", func(c *Context) {})
		tree._Insert(PUT, "/dashboard", func(c *Context) {})
		// 无panic即成功
	})

	// 4. 测试重复插入相同路由和方法
	t.Run("Duplicate Route-Method", func(t *testing.T) {
		// 首次插入
		tree._Insert(GET, "/settings", func(c *Context) {})
		// 重复插入应触发警告但无panic
		tree._Insert(GET, "/settings", func(c *Context) {})
	})

	// 5. 测试预期panic的错误路由
	t.Run("Panic Scenarios", func(t *testing.T) {
		// 5.1 星号通配不在末尾
		testPanic("Asterisk not last", func() {
			tree._Insert(GET, "/invalid/*/path", func(c *Context) {})
		})

		// 5.2 同一层级通配符冲突
		testPanic("Wildcard conflict", func() {
			tree._Insert(GET, "/conflict/:id", func(c *Context) {}) // 先插入正常路由
			tree._Insert(GET, "/conflict/*", func(c *Context) {})   // 冲突插入
		})

		// 5.3 无效冒号通配
		testPanic("Invalid colon wildcard", func() {
			tree._Insert(GET, "/invalid/:/empty", func(c *Context) {})
		})

		// 5.4 星号后还有路径
		testPanic("Asterisk with trailing parts", func() {
			tree._Insert(GET, "/*/invalid", func(c *Context) {})
		})
	})

	// 6. 测试混合路由场景
	t.Run("Complex Mixed Routes", func(t *testing.T) {
		routes := []struct {
			method Method
			path   string
		}{
			{GET, "/"},
			{GET, "/users"},
			{GET, "/users/:id"},
			{POST, "/users/:id"},
			{GET, "/users/:id/posts"},
			{GET, "/files/*"},
			{GET, "/:lang/articles"},
			{POST, "/:lang/articles"},
		}

		for _, route := range routes {
			tree._Insert(route.method, route.path, func(c *Context) {})
		}
		// 无panic即成功
	})

	tree.root.printAllRoutes()
}
