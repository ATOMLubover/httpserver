package util

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucket_BasicAllowance(t *testing.T) {
	// 初始化桶：容量=10个token, 速率=5 token/ms
	tb := NewTokenBucket(10, 5)

	// 首次请求应成功
	if !tb.Allow() {
		t.Error("首次请求应成功")
	}

	// 快速耗尽所有令牌
	for i := 0; i < 9; i++ {
		tb.Allow()
	}

	// 第11次请求应失败
	if tb.Allow() {
		t.Error("超过容量限制时应拒绝请求")
	}
}

func TestTokenBucket_RefillRate(t *testing.T) {
	tb := NewTokenBucket(2, 1) // 2 token容量, 1 token/ms
	tb.Allow()                 // 消耗1个
	tb.Allow()                 // 消耗1个

	// 此时桶为空
	start := time.Now()
	for !tb.Allow() { // 等待新令牌生成
		if time.Since(start) > 20*time.Millisecond {
			t.Fatal("等待令牌超时")
		}
	}

	// 验证生成时间应在1ms±10%范围内
	elapsed := time.Since(start)
	if elapsed < 900*time.Microsecond || elapsed > 1100*time.Microsecond {
		t.Errorf("令牌生成时间异常: %v", elapsed)
	}
}

func TestTokenBucket_EdgeCases(t *testing.T) {
	// 测试极小容量
	tb := NewTokenBucket(1, 1)
	if !tb.Allow() {
		t.Error("容量1时应允许首次请求")
	}
	if tb.Allow() {
		t.Error("容量1时应拒绝第二次请求")
	}

	// 测试极高容量(不触发溢出)
	tb = NewTokenBucket(math.MaxUint32/kDefaultCntPerToken, 1)
	for i := 0; i < 1000; i++ {
		if !tb.Allow() {
			t.Fatal("高容量下应允许请求")
		}
	}

	// 测试零速率
	tb = NewTokenBucket(1, 0)
	tb.Allow() // 消耗初始令牌
	if tb.Allow() {
		t.Error("零速率时应拒绝后续请求")
	}
}

func TestTokenBucket_TimePrecision(t *testing.T) {
	tb := NewTokenBucket(2, 1000) // 高刷新率

	// 验证时间重置保护
	tb.packedState.Store(sPackState(1, math.MaxInt32-10)) // 设置临近溢出的时间
	if !tb.Allow() {
		t.Error("时间重置保护失败")
	}
}

func TestTokenBucket_CustomOptions(t *testing.T) {
	// 设置非标准参数
	tb := NewTokenBucket(10, 5,
		WithCntPerToken(2),
		WithMaxAttempts(3),
		WithMaxAllowTime(100*time.Microsecond),
		WithMinExpBackoff(1),
	)

	// 消耗所有令牌
	for i := 0; i < 20; i++ {
		tb.Allow()
	}

	// 应快速返回false（因MaxAllowTime=100μs）
	start := time.Now()
	tb.Allow()
	if elapsed := time.Since(start); elapsed > 200*time.Microsecond {
		t.Errorf("超时时间配置未生效: %v", elapsed)
	}
}

func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	// 设置生成速率为0
	tb := NewTokenBucket(1000, 0)
	allowed := atomic.Int32{}

	var wg sync.WaitGroup
	workerCount := 2000
	wg.Add(workerCount)

	// 使用信号量控制并发的开始，确保所有goroutine同时开始请求
	start := make(chan struct{})

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			<-start // 等待开始信号

			if tb.Allow() {
				allowed.Add(1)
			}
		}()
	}

	// 同时启动所有goroutine
	close(start)
	wg.Wait()

	// 验证允许数量不超过容量
	if count := allowed.Load(); count != 1000 {
		t.Errorf("并发保护失败: allowed=%d, expected=1000", count)
	}
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	tb := NewTokenBucket(100000, 50000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tb.Allow()
	}
}
