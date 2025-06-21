package util

// import (
// 	"sort"
// 	"sync"
// 	"sync/atomic"
// 	"testing"
// 	"time"
// )

// // 测试顺序操作：入队和出队
// func TestMpmcLockfreeQueue_Sequential(t *testing.T) {
// 	q := NewMpmcLockfreeQueue[int](3)

// 	// 空队列出队应失败
// 	if _, ok := q.PopFront(); ok {
// 		t.Fatal("PopFront from empty queue should fail")
// 	}

// 	// 入队 1, 2, 3
// 	for i := 1; i <= 4; i++ {
// 		if !q.PushBack(i) {
// 			t.Fatalf("PushBack %d failed", i)
// 		}
// 	}

// 	// 队列已满，入队 4 应失败
// 	if q.PushBack(5) {
// 		t.Fatal("PushBack should fail when queue is full")
// 	}

// 	// 出队顺序应为 1, 2, 3
// 	for i := 1; i <= 4; i++ {
// 		item, ok := q.PopFront()
// 		if !ok || item != i {
// 			t.Fatalf("PopFront %d failed, got %v", i, item)
// 		}
// 	}

// 	// 再次出队应失败
// 	if _, ok := q.PopFront(); ok {
// 		t.Fatal("PopFront after all items should fail")
// 	}
// }

// // 测试队列容量函数
// func TestMpmcLockfreeQueue_Capacity(t *testing.T) {
// 	q := NewMpmcLockfreeQueue[int](10)
// 	if cap := q.Capacity(); cap != 16 { // 大于10的最小2的幂是16
// 		t.Fatalf("Expected capacity 16, got %d", cap)
// 	}
// }

// // 并发测试：多生产者多消费者
// func TestMpmcLockfreeQueue_Concurrent(t *testing.T) {
// 	const (
// 		producers    = 4   // 生产者数量
// 		consumers    = 4   // 消费者数量
// 		itemsPerProd = 100 // 每个生产者生产的数量
// 		queueCap     = 128 // 队列容量（2的幂）
// 	)

// 	q := NewMpmcLockfreeQueue[int](queueCap)
// 	totalItems := producers * itemsPerProd

// 	// 用于收集消费结果的带缓冲通道
// 	resultChan := make(chan int, totalItems)

// 	var wg sync.WaitGroup

// 	// 启动生产者
// 	wg.Add(producers)
// 	for i := 0; i < producers; i++ {
// 		go func(start int) {
// 			defer wg.Done()
// 			for j := 0; j < itemsPerProd; j++ {
// 				item := start + j       // 生成唯一值
// 				for !q.PushBack(item) { // 重试直到成功
// 				}
// 			}
// 		}(i * itemsPerProd)
// 	}

// 	// 启动消费者
// 	wg.Add(consumers)
// 	for i := 0; i < consumers; i++ {
// 		go func() {
// 			defer wg.Done()
// 			for j := 0; j < itemsPerProd; j++ {
// 				for {
// 					if item, ok := q.PopFront(); ok {
// 						resultChan <- item
// 						break
// 					}
// 				}
// 			}
// 		}()
// 	}

// 	wg.Wait() // 等待所有协程完成
// 	close(resultChan)

// 	// 收集并验证结果
// 	results := make([]int, 0, totalItems)
// 	for item := range resultChan {
// 		results = append(results, item)
// 	}

// 	if len(results) != totalItems {
// 		t.Fatalf("Expected %d items, got %d", totalItems, len(results))
// 	}

// 	// 排序并验证每个值唯一性
// 	sort.Ints(results)
// 	for i := 0; i < totalItems; i++ {
// 		if results[i] != i {
// 			t.Fatalf("Missing or duplicate value: expected %d, got %d", i, results[i])
// 		}
// 	}
// }

// // 测试队列在容量不足时的重试行为
// func TestMpmcLockfreeQueue_RetryWhenFull(t *testing.T) {
// 	q := NewMpmcLockfreeQueue[int](2) // 容量为2

// 	// 快速填充队列
// 	q.PushBack(1)
// 	q.PushBack(2)

// 	// 并发尝试添加第3项（需重试）
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		for !q.PushBack(3) { // 此操作需等待消费者移除元素
// 			//t.Error("Failed to push after retry")
// 		}
// 	}()

// 	// 稍后移除一个元素以腾出空间
// 	time.Sleep(100 * time.Millisecond)
// 	if _, ok := q.PopFront(); !ok {
// 		t.Fatal("PopFront should succeed")
// 	}

// 	wg.Wait() // 确保生产者完成

// 	// 验证新元素已入
// 	if item, ok := q.PopFront(); !ok || item != 2 {
// 		t.Fatalf("Expected 2, got %v", item)
// 	}
// 	if item, ok := q.PopFront(); !ok || item != 3 {
// 		t.Fatalf("Expected 3, got %v", item)
// 	}
// }

// // 测试空队列处理
// func TestMpmcLockfreeQueue_EmptyQueue(t *testing.T) {
// 	q := NewMpmcLockfreeQueue[string](2)

// 	// 尝试从空队列弹出
// 	if val, ok := q.PopFront(); ok {
// 		t.Fatalf("Expected failure, got value: %s", val)
// 	}

// 	// 添加后移除
// 	q.PushBack("test")
// 	if val, ok := q.PopFront(); !ok || val != "test" {
// 		t.Fatal("PopFront failed after push")
// 	}

// 	// 再次尝试弹出（应失败）
// 	if val, ok := q.PopFront(); ok {
// 		t.Fatalf("Expected failure after emptying, got: %s", val)
// 	}
// }

// // 测试队列满状态
// func TestMpmcLockfreeQueue_FullQueue(t *testing.T) {
// 	q := NewMpmcLockfreeQueue[rune](2)

// 	// 填充队列
// 	q.PushBack('a')
// 	q.PushBack('b')

// 	// 尝试添加第三个元素
// 	if q.PushBack('c') {
// 		t.Fatal("PushBack should fail when queue is full")
// 	}

// 	// 弹出一个元素后尝试添加
// 	q.PopFront()
// 	if !q.PushBack('c') {
// 		t.Fatal("PushBack should succeed after popping")
// 	}

// 	// 验证内容
// 	if val, _ := q.PopFront(); val != 'b' {
// 		t.Fatal("Unexpected value")
// 	}
// 	if val, _ := q.PopFront(); val != 'c' {
// 		t.Fatal("Unexpected value")
// 	}
// }

// // 基准测试：评估并发性能
// func BenchmarkMpmcLockfreeQueue_Concurrent(b *testing.B) {
// 	q := NewMpmcLockfreeQueue[int](1024)
// 	var counter int64

// 	b.ResetTimer()
// 	b.RunParallel(func(pb *testing.PB) {
// 		for pb.Next() {
// 			// 混合执行入队/出队
// 			idx := atomic.AddInt64(&counter, 1)
// 			if idx%2 == 0 {
// 				q.PushBack(int(idx))
// 			} else {
// 				q.PopFront()
// 			}
// 		}
// 	})
// }

// import (
// 	"sync"
// 	"testing"
// 	"time"
// )

// func TestMpmcLockfreeQueue(t *testing.T) {
// 	t.Run("BasicOperations", func(t *testing.T) {
// 		q := NewMpmcLockfreeQueue[int](4)
// 		actualCap := q.Capacity() // 获取实际容量

// 		// 测试空队列弹出
// 		if _, ok := q.PopFront(); ok {
// 			t.Error("Empty queue should pop fail")
// 		}

// 		// 测试单元素插入和弹出
// 		if !q.PushBack(42) {
// 			t.Error("Push should succeed")
// 		}
// 		if val, ok := q.PopFront(); !ok || val != 42 {
// 			t.Errorf("Pop failed, expected 42, got %v", val)
// 		}

// 		// 测试队列填满（使用实际容量）
// 		for i := 0; i < int(actualCap); i++ {
// 			if !q.PushBack(i) {
// 				t.Errorf("Push %d failed", i)
// 			}
// 		}
// 		if q.PushBack(99) {
// 			t.Error("Full queue should not accept new items")
// 		}

// 		// 测试队列清空
// 		for i := 0; i < int(actualCap); i++ {
// 			if val, ok := q.PopFront(); !ok || val != i {
// 				t.Errorf("Pop failed at %d, got %v", i, val)
// 			}
// 		}
// 		if _, ok := q.PopFront(); ok {
// 			t.Error("Emptied queue should pop fail")
// 		}
// 	})

// 	t.Run("Concurrency", func(t *testing.T) {
// 		const (
// 			capacity     = 100
// 			producers    = 4
// 			consumers    = 4
// 			itemsPerProd = 1000
// 			totalItems   = producers * itemsPerProd
// 			timeout      = 10 * time.Second
// 		)

// 		q := NewMpmcLockfreeQueue[int](capacity)
// 		var wg sync.WaitGroup
// 		results := make(chan int, totalItems)
// 		start := make(chan struct{})

// 		// 生产者协程 (修复：避免指针覆盖)
// 		for i := 0; i < producers; i++ {
// 			wg.Add(1)
// 			go func(id int) {
// 				defer wg.Done()
// 				<-start

// 				for j := 0; j < itemsPerProd; j++ {
// 					item := id*itemsPerProd + j
// 					// 创建新指针避免数据竞争
// 					pItem := new(int)
// 					*pItem = item
// 					for !q.PushBack(*pItem) {
// 						time.Sleep(time.Microsecond)
// 					}
// 				}
// 			}(i)
// 		}

// 		// 消费者协程
// 		for i := 0; i < consumers; i++ {
// 			wg.Add(1)
// 			go func() {
// 				defer wg.Done()
// 				<-start

// 				for count := 0; count < totalItems/consumers; count++ {
// 					for {
// 						if val, ok := q.PopFront(); ok {
// 							results <- val // 解引用获取值
// 							break
// 						}
// 						time.Sleep(time.Microsecond)
// 					}
// 				}
// 			}()
// 		}

// 		// 启动所有协程
// 		close(start)
// 		done := make(chan struct{})
// 		go func() {
// 			wg.Wait()
// 			close(done)
// 		}()

// 		// 等待结果或超时
// 		select {
// 		case <-done:
// 		case <-time.After(timeout):
// 			t.Fatal("Test timed out")
// 		}

// 		close(results)

// 		// 验证结果
// 		received := make(map[int]bool)
// 		for item := range results {
// 			if received[item] {
// 				t.Errorf("Duplicate item found: %d", item)
// 			}
// 			received[item] = true
// 		}

// 		if len(received) != totalItems {
// 			t.Errorf("Missing items, expected %d, got %d", totalItems, len(received))
// 		}

// 		// 验证队列为空
// 		if _, ok := q.PopFront(); ok {
// 			t.Error("Queue should be empty after test")
// 		}
// 	})

// 	t.Run("PowerOfTwoCapacity", func(t *testing.T) {
// 		testCases := []struct {
// 			input    int64
// 			expected int64
// 		}{
// 			{1, 1},
// 			{3, 4},
// 			{5, 8},
// 			{100, 128},
// 			{1024, 1024},
// 		}

// 		for _, tc := range testCases {
// 			q := NewMpmcLockfreeQueue[int](tc.input)
// 			if q.capacity != tc.expected {
// 				t.Errorf("For input %d, expected capacity %d, got %d",
// 					tc.input, tc.expected, q.capacity)
// 			}
// 		}
// 	})

// 	t.Run("MixedOperations", func(t *testing.T) {
// 		q := NewMpmcLockfreeQueue[int](10)
// 		var wg sync.WaitGroup

// 		wg.Add(2)
// 		// 持续生产者
// 		go func() {
// 			defer wg.Done()
// 			for i := 0; i < 1000; i++ {
// 				for !q.PushBack(i) {
// 					time.Sleep(time.Microsecond)
// 				}
// 			}
// 		}()

// 		// 持续消费者
// 		go func() {
// 			defer wg.Done()
// 			for i := 0; i < 1000; i++ {
// 				for {
// 					if _, ok := q.PopFront(); ok {
// 						break
// 					}
// 					time.Sleep(time.Microsecond)
// 				}
// 			}
// 		}()

// 		wg.Wait()

// 		// 最终队列应为空
// 		if _, ok := q.PopFront(); ok {
// 			t.Error("Queue should be empty after mixed operations")
// 		}
// 	})
// }
