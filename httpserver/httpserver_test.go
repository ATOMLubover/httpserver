package httpserver

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

// func TestRouteTreeInsert(t *testing.T) {
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

// 	// 创建路由树实例
// 	tree := _NewRouteTree()

// 	// 辅助函数：安全执行可能panic的操作
// 	testPanic := func(name string, f func()) {
// 		defer func() {
// 			if r := recover(); r != nil {
// 				t.Logf("%s: got expected panic: %v", name, r)
// 			}
// 		}()
// 		f()
// 	}

// 	// 1. 测试普通路由插入
// 	t.Run("Normal Routes", func(t *testing.T) {
// 		tree._Insert(GET, "/api/users", func(c *Context) {})
// 		tree._Insert(POST, "/api/users", func(c *Context) {})
// 		tree._Insert(GET, "/api/products", func(c *Context) {})
// 		// 无panic即成功
// 	})

// 	// 2. 测试通配路由
// 	t.Run("Wildcard Routes", func(t *testing.T) {
// 		// 冒号通配
// 		tree._Insert(GET, "/users/:id", func(c *Context) {})
// 		tree._Insert(GET, "/users/:id/profile", func(c *Context) {})

// 		// 星号通配
// 		tree._Insert(GET, "/static/*", func(c *Context) {})
// 		// 无panic即成功
// 	})

// 	// 3. 测试同一路由不同方法
// 	t.Run("Same Route Different Methods", func(t *testing.T) {
// 		tree._Insert(GET, "/dashboard", func(c *Context) {})
// 		tree._Insert(POST, "/dashboard", func(c *Context) {})
// 		tree._Insert(PUT, "/dashboard", func(c *Context) {})
// 		// 无panic即成功
// 	})

// 	// 4. 测试重复插入相同路由和方法
// 	t.Run("Duplicate Route-Method", func(t *testing.T) {
// 		// 首次插入
// 		tree._Insert(GET, "/settings", func(c *Context) {})
// 		// 重复插入应触发警告但无panic
// 		tree._Insert(GET, "/settings", func(c *Context) {})
// 	})

// 	// 5. 测试预期panic的错误路由
// 	t.Run("Panic Scenarios", func(t *testing.T) {
// 		// 5.1 星号通配不在末尾
// 		testPanic("Asterisk not last", func() {
// 			tree._Insert(GET, "/invalid/*/path", func(c *Context) {})
// 		})

// 		// 5.2 同一层级通配符冲突
// 		testPanic("Wildcard conflict", func() {
// 			tree._Insert(GET, "/conflict/:id", func(c *Context) {}) // 先插入正常路由
// 			tree._Insert(GET, "/conflict/*", func(c *Context) {})   // 冲突插入
// 		})

// 		// 5.3 无效冒号通配
// 		testPanic("Invalid colon wildcard", func() {
// 			tree._Insert(GET, "/invalid/:/empty", func(c *Context) {})
// 		})

// 		// 5.4 星号后还有路径
// 		testPanic("Asterisk with trailing parts", func() {
// 			tree._Insert(GET, "/*/invalid", func(c *Context) {})
// 		})
// 	})

// 	// 6. 测试混合路由场景
// 	t.Run("Complex Mixed Routes", func(t *testing.T) {
// 		routes := []struct {
// 			method Method
// 			path   string
// 		}{
// 			{GET, "/"},
// 			{GET, "/users"},
// 			{GET, "/users/:id"},
// 			{POST, "/users/:id"},
// 			{GET, "/users/:id/posts"},
// 			{GET, "/files/*"},
// 			{GET, "/:lang/articles"},
// 			{POST, "/:lang/articles"},
// 		}

// 		for _, route := range routes {
// 			tree._Insert(route.method, route.path, func(c *Context) {})
// 		}
// 		// 无panic即成功
// 	})

// 	tree.root.printAllRoutes()
// }

// 测试路由组的基本路由插入功能
// func TestRouteGroupAddRoute(t *testing.T) {
// 	// 创建 HandlerOptions 并设置日志级别为 Debug
// 	handlerOpts := &slog.HandlerOptions{
// 		Level: slog.LevelDebug, // 设置日志级别为 Debug
// 	}

// 	// 创建 JSON 格式的 Handler
// 	loghandler := slog.NewTextHandler(os.Stdout, handlerOpts)

// 	// 创建新的 Logger
// 	logger := slog.New(loghandler)

// 	// 设置为全局默认 Logger
// 	slog.SetDefault(logger)

// 	// 创建路由树和根路由组
// 	tree := _NewRouteTree()
// 	rootGroup := &RouteGroup{
// 		routeTree:   tree,
// 		prefix:      "/api/",
// 		middlewares: []MiddlewareFunc{},
// 	}

// 	// 定义测试处理器
// 	handler := func(c *Context) {}

// 	// 测试用例：有效的路由插入
// 	t.Run("Valid routes", func(t *testing.T) {
// 		// 插入普通路由
// 		rootGroup.AddRoute(GET, "/users", handler)
// 		rootGroup.AddRoute(POST, "/products", handler)

// 		// 插入通配路由
// 		rootGroup.AddRoute(GET, "/users/:id", handler)
// 		rootGroup.AddRoute(GET, "/static/*", handler)

// 		// 无panic即成功
// 	})

// 	// 测试用例：无效的路由插入（预期panic）
// 	t.Run("Invalid routes", func(t *testing.T) {
// 		defer func() {
// 			if r := recover(); r == nil {
// 				t.Error("Expected panic for invalid route, but none occurred")
// 			}
// 		}()

// 		// 无效模式（缺少前导斜杠）
// 		rootGroup.AddRoute(GET, "invalid", handler)
// 	})
// }

// // 测试子路由组创建和路由插入
// func TestRouteGroupAddGroup(t *testing.T) {
// 	// 创建路由树和根路由组
// 	tree := _NewRouteTree()
// 	rootGroup := &RouteGroup{
// 		routeTree:   tree,
// 		prefix:      "/api/",
// 		middlewares: []MiddlewareFunc{},
// 	}

// 	// 定义测试处理器
// 	handler := func(c *Context) {}

// 	// 创建子路由组
// 	userGroup := rootGroup.AddGroup("/users/")
// 	adminGroup := rootGroup.AddGroup("/admin/")

// 	// 在子路由组中添加路由
// 	t.Run("Add routes to subgroups", func(t *testing.T) {
// 		userGroup.AddRoute(GET, "/profile", handler)
// 		userGroup.AddRoute(GET, "/:id", handler)
// 		adminGroup.AddRoute(GET, "/dashboard", handler)
// 		adminGroup.AddRoute(POST, "/settings", handler)

// 		// 无panic即成功
// 	})

// 	// 测试子路由组的前缀组合
// 	t.Run("Verify subgroup prefixes", func(t *testing.T) {
// 		if userGroup.prefix != "/api/users/" {
// 			t.Errorf("Expected user group prefix '/api/users/', got '%s'", userGroup.prefix)
// 		}

// 		if adminGroup.prefix != "/api/admin/" {
// 			t.Errorf("Expected admin group prefix '/api/admin/', got '%s'", adminGroup.prefix)
// 		}
// 	})

// 	// 测试无效子路由组创建（预期panic）
// 	t.Run("Invalid subgroup creation", func(t *testing.T) {
// 		defer func() {
// 			if r := recover(); r == nil {
// 				t.Error("Expected panic for invalid group prefix, but none occurred")
// 			}
// 		}()

// 		// 无效前缀（包含通配符）
// 		rootGroup.AddGroup("/invalid/*/")
// 	})

//     tree.root.printAllRoutes()
// }

// // 测试路由组中间件功能
// func TestRouteGroupMiddleware(t *testing.T) {
// 	// 创建路由树和根路由组
// 	tree := _NewRouteTree()
// 	rootGroup := &RouteGroup{
// 		routeTree:   tree,
// 		prefix:      "/api/",
// 		middlewares: []MiddlewareFunc{},
// 	}

// 	// 创建中间件调用记录器
// 	calls := []string{}
// 	recordCall := func(name string) MiddlewareFunc {
// 		return func(next HandlerFunc) HandlerFunc {
// 			return func(c *Context) {
// 				calls = append(calls, name)
// 				next(c)
// 			}
// 		}
// 	}

// 	// 在根路由组添加中间件
// 	rootGroup.UseMiddleware(recordCall("root-middleware1"))
// 	rootGroup.UseMiddleware(recordCall("root-middleware2"))

// 	// 创建子路由组并添加中间件
// 	userGroup := rootGroup.AddGroup("/users/")
// 	userGroup.UseMiddleware(recordCall("user-middleware"))

// 	// 在子路由组中添加路由
// 	handler := func(c *Context) {
// 		calls = append(calls, "handler")
// 	}
// 	userGroup.AddRoute(GET, "/profile", handler)

// 	// 模拟调用处理器（执行中间件链）
// 	t.Run("Middleware execution order", func(t *testing.T) {
// 		// 重置调用记录
// 		calls = []string{}

// 		// 执行处理器（包含所有中间件）
// 		// 注意：实际框架中这会由路由引擎调用，这里模拟调用
// 		wrappedHandler := handler
// 		for i := len(userGroup.middlewares) - 1; i >= 0; i-- {
// 			wrappedHandler = userGroup.middlewares[i](wrappedHandler)
// 		}
// 		wrappedHandler(&Context{}) // 执行包装后的处理器链

// 		// 验证调用顺序
// 		expected := []string{
// 			"root-middleware1",
// 			"root-middleware2",
// 			"user-middleware",
// 			"handler",
// 		}

// 		if len(calls) != len(expected) {
// 			t.Fatalf("Expected %d calls, got %d", len(expected), len(calls))
// 		}

// 		for i, call := range calls {
// 			if call != expected[i] {
// 				t.Errorf("Call %d: expected '%s', got '%s'", i, expected[i], call)
// 			}
// 		}
// 	})

// 	// 测试中间件继承
// 	t.Run("Middleware inheritance", func(t *testing.T) {
// 		// 创建孙子路由组
// 		prefsGroup := userGroup.AddGroup("/prefs/")

// 		// 应继承所有父级中间件
// 		if len(prefsGroup.middlewares) != 3 {
// 			t.Errorf("Expected 3 inherited middlewares, got %d", len(prefsGroup.middlewares))
// 		}
// 	})
// }

func createMockMiddleware(id int) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			// 可选：记录日志或断言 id 值
			// log.Printf("Middleware %d executed", id)
			next(ctx)
		}
	}
}

// // 测试基本功能
// func TestCacheBasicOperations(t *testing.T) {
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

// 	cache := _NewMiddlewareChainCache(5)
// 	defer close(cache.closeChan)

// 	// 创建函数计数器
// 	creationCounter := 0
// 	creationFunc := func() []MiddlewareFunc {
// 		creationCounter++
// 		return []MiddlewareFunc{createMockMiddleware(creationCounter)}
// 	}

// 	// 首次获取 - 应该创建新条目
// 	chain := cache._Get("key1", creationFunc)
// 	if len(chain) != 1 {
// 		t.Fatalf("Expected chain length 1, got %d", len(chain))
// 	}
// 	if creationCounter != 1 {
// 		t.Fatalf("Expected creationFunc called once, got %d", creationCounter)
// 	}

// 	// 再次获取相同key - 应该命中缓存
// 	chain = cache._Get("key1", creationFunc)
// 	if len(chain) != 1 {
// 		t.Fatalf("Expected chain length 1, got %d", len(chain))
// 	}
// 	if creationCounter != 1 {
// 		t.Fatalf("Expected no additional creation, got %d", creationCounter)
// 	}

// 	// 获取不同key - 应该创建新条目
// 	chain = cache._Get("key2", creationFunc)
// 	if len(chain) != 1 {
// 		t.Fatalf("Expected chain length 1, got %d", len(chain))
// 	}
// 	if creationCounter != 2 {
// 		t.Fatalf("Expected creationFunc called twice, got %d", creationCounter)
// 	}
// }

// // 测试LRU淘汰逻辑
// func TestCacheEviction(t *testing.T) {
// 	cache := _NewMiddlewareChainCache(3)
// 	defer close(cache.closeChan)

// 	// 填充缓存
// 	for i := 0; i < 4; i++ {
// 		key := "key" + strconv.Itoa(i)
// 		cache._Get(key, func() []MiddlewareFunc {
// 			return []MiddlewareFunc{createMockMiddleware(i)}
// 		})
// 	}

// 	// 验证缓存大小
// 	if cache.lruList.Len() != 3 {
// 		t.Fatalf("Expected cache size 3, got %d", cache.lruList.Len())
// 	}

// 	// 验证第一个key被淘汰
// 	if _, exists := cache.entries["key0"]; exists {
// 		t.Fatal("Expected key0 to be evicted")
// 	}

// 	// 验证最新key存在
// 	if _, exists := cache.entries["key3"]; !exists {
// 		t.Fatal("Expected key3 to be present")
// 	}
// }

// 测试脏标记和重建逻辑
func TestDirtyMarkingAndRebuild(t *testing.T) {
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

	cache := _NewMiddlewareChainCache(5)
	defer close(cache.closeChan)

	// 添加条目
	cache._Get("key1", func() []MiddlewareFunc {
		return []MiddlewareFunc{createMockMiddleware(1)}
	})
	cache._Get("key2", func() []MiddlewareFunc {
		return []MiddlewareFunc{createMockMiddleware(2)}
	})

	// 访问key1使其变脏
	cache._TryGettingInCache("key1")

	// 验证脏标记
	func() {
		elem := cache.entries["key1"]
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		entry.globalMutex.Lock()
		defer entry.globalMutex.Unlock()
		if !entry.isDirty {
			t.Fatal("Expected key1 to be dirty")
		}
	}()

	// 验证脏列表
	if cache.dirtyList.Len() != 1 {
		t.Fatalf("Expected 1 dirty entry, got %d", cache.dirtyList.Len())
	}

	// 手动触发重建
	cache._RebuildOrder()

	// 验证重建后状态
	if cache.dirtyList.Len() != 0 {
		t.Fatalf("Expected 0 dirty entries after rebuild, got %d", cache.dirtyList.Len())
	}

	// 验证key1不再脏
	func() {
		elem := cache.entries["key1"]
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		entry.globalMutex.Lock()
		defer entry.globalMutex.Unlock()
		if entry.isDirty {
			t.Fatal("Expected key1 to be clean after rebuild")
		}
	}()
}

// 测试并发访问 - 基本功能
func TestConcurrentAccess(t *testing.T) {
	cache := _NewMiddlewareChainCache(10)
	defer close(cache.closeChan)

	var wg sync.WaitGroup
	creationCounter := atomic.Int32{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(id%5) // 5个不同的key
			cache._Get(key, func() []MiddlewareFunc {
				creationCounter.Add(1)
				time.Sleep(10 * time.Millisecond) // 模拟创建延迟
				return []MiddlewareFunc{createMockMiddleware(id)}
			})
		}(i)
	}

	wg.Wait()

	// 验证每个key只创建一次
	if creationCounter.Load() != 5 {
		t.Fatalf("Expected 5 creations, got %d", creationCounter.Load())
	}

	// 验证缓存统计
	if cache.hits.Load() == 0 {
		t.Fatal("Expected some cache hits")
	}
	if cache.misses.Load() != 5 {
		t.Fatalf("Expected 5 misses, got %d", cache.misses.Load())
	}
}

// 测试缓存击穿防护
func TestCacheStampedeProtection(t *testing.T) {
	cache := _NewMiddlewareChainCache(5)
	defer close(cache.closeChan)

	const key = "testKey"
	const concurrentRequests = 50

	var (
		wg             sync.WaitGroup
		creationCount  atomic.Int32
		firstCreatedAt time.Time
		creationTimes  = make([]time.Time, concurrentRequests)
	)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			chain := cache._Get(key, func() []MiddlewareFunc {
				count := creationCount.Add(1)
				if count == 1 {
					firstCreatedAt = time.Now()
				}

				// 模拟创建耗时
				time.Sleep(100 * time.Millisecond)
				return []MiddlewareFunc{createMockMiddleware(id)}
			})
			slog.Debug(fmt.Sprintf("Got chain(%v) for key %s, id %d", chain, key, id))
			creationTimes[id] = time.Now()
		}(i)
	}

	wg.Wait()

	// 验证只创建了一次
	if creationCount.Load() != 1 {
		t.Fatalf("Expected only 1 creation, got %d", creationCount.Load())
	}

	// 验证所有goroutine都获得相同结果
	for i, ct := range creationTimes {
		if ct.Before(firstCreatedAt.Add(90 * time.Millisecond)) {
			t.Fatalf("Request %d returned too early, before creation finished", i)
		}
	}
}

// 测试高并发重建
func TestConcurrentRebuild(t *testing.T) {
	cache := _NewMiddlewareChainCache(20)
	defer close(cache.closeChan)

	// 填充缓存
	for i := 0; i < 20; i++ {
		key := "key" + strconv.Itoa(i)
		cache._Get(key, func() []MiddlewareFunc {
			return []MiddlewareFunc{createMockMiddleware(i)}
		})
	}

	// 启动重建
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// cache.globalMutex.Lock()
		// defer cache.globalMutex.Unlock()
		cache._RebuildOrder()
	}()

	// 在重建过程中并发访问
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(id%20)
			chain := cache._Get(key, func() []MiddlewareFunc {
				return []MiddlewareFunc{createMockMiddleware(id)}
			})
			if len(chain) == 0 {
				t.Error("Got empty chain")
			}
		}(i)
	}

	wg.Wait()
}

// 测试缓存关闭
func TestCacheClose(t *testing.T) {
	cache := _NewMiddlewareChainCache(5)

	// 启动后台重建
	closed := make(chan bool)
	go func() {
		cache._BackgroundRebuild()
		closed <- true
	}()

	// 添加一些数据
	cache._Get("key1", func() []MiddlewareFunc {
		return []MiddlewareFunc{createMockMiddleware(1)}
	})

	// 关闭缓存
	close(cache.closeChan)

	select {
	case <-closed:
		// 正常关闭
	case <-time.After(1 * time.Second):
		t.Fatal("Background goroutine did not exit")
	}
}

// 压力测试 - 高并发访问
func BenchmarkCacheUnderLoad(b *testing.B) {
	cache := _NewMiddlewareChainCache(100)
	defer close(cache.closeChan)

	// 预填充缓存
	for i := 0; i < 100; i++ {
		key := "key" + strconv.Itoa(i)
		cache._Get(key, func() []MiddlewareFunc {
			return []MiddlewareFunc{createMockMiddleware(i)}
		})
	}

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			counter++
			key := "key" + strconv.Itoa(counter%150) // 100个缓存key + 50个未缓存
			cache._Get(key, func() []MiddlewareFunc {
				time.Sleep(5 * time.Millisecond) // 模拟创建开销
				return []MiddlewareFunc{createMockMiddleware(counter)}
			})
		}
	})

	b.ReportMetric(float64(cache.hits.Load())/float64(b.N), "hits/op")
	b.ReportMetric(float64(cache.misses.Load())/float64(b.N), "misses/op")
}
