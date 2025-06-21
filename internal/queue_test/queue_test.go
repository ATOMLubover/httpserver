package queue_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ATOMLubover/httpserver/internal/util" // 替换为你的无锁队列包路径
)

const (
	totalOperations = 1_000_000 // 每次测试总操作数
	warmupRuns      = 3         // 预热运行次数
)

// 测试负载结构体
type testPayload struct {
	ID      int
	Value   float64
	Message string
}

// 基准测试函数
func runBenchmark(b *testing.B, queue interface {
	PushBack(testPayload) bool
	PopFront() (testPayload, bool)
}, producers, consumers int) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		start := time.Now()
		ops := int64(0)

		// 启动生产者
		for p := 0; p < producers; p++ {
			wg.Add(1)
			go func(pid int) {
				defer wg.Done()
				for i := 0; i < totalOperations/producers; i++ {
					payload := testPayload{
						ID:      i,
						Value:   float64(i) * 1.618,
						Message: "Hello from producer",
					}
					for !queue.PushBack(payload) {
						// 队列满时重试
					}
				}
			}(p)
		}

		// 启动消费者
		for c := 0; c < consumers; c++ {
			wg.Add(1)
			go func(cid int) {
				defer wg.Done()
				for i := 0; i < totalOperations/consumers; i++ {
					var result testPayload
					var ok bool
					for {
						result, ok = queue.PopFront()
						if ok {
							break
						}
						// 队列空时重试
					}
					_ = result // 防止编译器优化
					atomic.AddInt64(&ops, 1)
				}
			}(c)
		}

		wg.Wait()
		duration := time.Since(start)
		b.ReportMetric(float64(ops)/float64(duration.Nanoseconds()), "ops/ns")
	}
}

// 无锁队列包装器
type lockfreeQueueWrapper struct {
	q *util.MpmcLockfreeQueue[testPayload]
}

func (lq *lockfreeQueueWrapper) PushBack(p testPayload) bool {
	return lq.q.PushBack(p)
}

func (lq *lockfreeQueueWrapper) PopFront() (testPayload, bool) {
	return lq.q.PopFront()
}

// Channel包装器
type channelWrapper struct {
	ch chan testPayload
}

func (cw *channelWrapper) PushBack(p testPayload) bool {
	select {
	case cw.ch <- p:
		return true
	default:
		return false
	}
}

func (cw *channelWrapper) PopFront() (testPayload, bool) {
	select {
	case p := <-cw.ch:
		return p, true
	default:
		return testPayload{}, false
	}
}

// ========== 无锁队列基准测试 ==========

func BenchmarkLockfreeQueue_1P1C_Size16(b *testing.B) {
	q := &lockfreeQueueWrapper{
		q: util.NewMpmcLockfreeQueue[testPayload](16),
	}
	runBenchmark(b, q, 1, 1)
}

func BenchmarkLockfreeQueue_2P2C_Size16(b *testing.B) {
	q := &lockfreeQueueWrapper{
		q: util.NewMpmcLockfreeQueue[testPayload](16),
	}
	runBenchmark(b, q, 2, 2)
}

func BenchmarkLockfreeQueue_4P4C_Size16(b *testing.B) {
	q := &lockfreeQueueWrapper{
		q: util.NewMpmcLockfreeQueue[testPayload](16),
	}
	runBenchmark(b, q, 4, 4)
}

func BenchmarkLockfreeQueue_8P8C_Size16(b *testing.B) {
	q := &lockfreeQueueWrapper{
		q: util.NewMpmcLockfreeQueue[testPayload](16),
	}
	runBenchmark(b, q, 8, 8)
}

func BenchmarkLockfreeQueue_8P8C_Size1024(b *testing.B) {
	q := &lockfreeQueueWrapper{
		q: util.NewMpmcLockfreeQueue[testPayload](1024),
	}
	runBenchmark(b, q, 8, 8)
}

// ========== Channel基准测试 ==========

func BenchmarkChannel_1P1C_Size16(b *testing.B) {
	q := &channelWrapper{
		ch: make(chan testPayload, 16),
	}
	runBenchmark(b, q, 1, 1)
}

func BenchmarkChannel_2P2C_Size16(b *testing.B) {
	q := &channelWrapper{
		ch: make(chan testPayload, 16),
	}
	runBenchmark(b, q, 2, 2)
}

func BenchmarkChannel_4P4C_Size16(b *testing.B) {
	q := &channelWrapper{
		ch: make(chan testPayload, 16),
	}
	runBenchmark(b, q, 4, 4)
}

func BenchmarkChannel_8P8C_Size16(b *testing.B) {
	q := &channelWrapper{
		ch: make(chan testPayload, 16),
	}
	runBenchmark(b, q, 8, 8)
}

func BenchmarkChannel_16P16C_Size1024(b *testing.B) {
	q := &channelWrapper{
		ch: make(chan testPayload, 1024),
	}
	runBenchmark(b, q, 16, 16)
}
