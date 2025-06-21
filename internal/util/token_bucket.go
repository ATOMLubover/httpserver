package util

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// Function option for configuring a token bucket.
type Option func(*TokenBucket)

// Default values for token bucket options.
const (
	kDefaultCntPerToken   uint32        = 1 << 20
	kDefaultMaxAttempts   int           = 5
	kDefaultMaxAllowTime  time.Duration = time.Millisecond
	kDefaultMinExpBackoff int           = 2
	kDefaultTokenNumShift uint          = 32
)

// Unpack the state of the token bucket.
func sUnpackState(state uint64) (tokenCnt uint32, lastRelTime int32) {
	const kTimeMask = 1<<kDefaultTokenNumShift - 1
	return uint32(state >> kDefaultTokenNumShift), int32(state & kTimeMask)
}

// Pack the state of the token bucket.
func sPackState(tokenCnt uint32, lastRelTime int32) uint64 {
	return uint64(tokenCnt)<<kDefaultTokenNumShift | uint64(lastRelTime)
}

// Token bucket algorithm based rate limiter.
type TokenBucket struct {
	// State of the bucket.
	// (upper 32: token cnt, lower 32: last update time)
	packedState atomic.Uint64

	// Max size of bucket.
	capacity uint32
	// Rate of generating tokens.
	// Its unit is count per milliseconds.
	cntGenRate uint32

	// When the bucket is created at.
	baseTime int64

	cntPerToken   uint32
	maxAttempts   int
	maxAllowTime  time.Duration
	minExpBackoff int
}

// Set the count per token.
// The bigger the count is, the smaller capacity of bucket can reach.
func WithCntPerToken(cnt uint32) Option {
	return func(tb *TokenBucket) {
		tb.cntPerToken = cnt
	}
}

// Set the maximum number of attempts.
func WithMaxAttempts(attemptCnt int) Option {
	return func(tb *TokenBucket) {
		tb.maxAttempts = attemptCnt
	}
}

// Set the minimum exponential backoff.
func WithMaxAllowTime(maxDuration time.Duration) Option {
	return func(tb *TokenBucket) {
		tb.maxAllowTime = maxDuration
	}
}

// Set the minimum time after which exponential backoff will applied.
// It should not be less than max attempts.
func WithMinExpBackoff(iterCnt int) Option {
	return func(tb *TokenBucket) {
		tb.minExpBackoff = iterCnt
	}
}

// Create a new token bucket with functional options.
// genRate is the number of tokens generated per millisecond.
// cap and genRate have limits according to the contant params.
func NewTokenBucket(cap uint32, genRate uint32, opts ...Option) *TokenBucket {
	bucket := &TokenBucket{
		cntPerToken:   kDefaultCntPerToken,
		maxAttempts:   kDefaultMaxAttempts,
		maxAllowTime:  kDefaultMaxAllowTime,
		minExpBackoff: kDefaultMinExpBackoff,
	}

	// Apply all options.
	for _, opt := range opts {
		opt(bucket)
	}

	// Then, calculate the rest properties based on these options.
	cap = min(cap, math.MaxUint32/bucket.cntPerToken)
	genRate = min(genRate, math.MaxUint32/bucket.cntPerToken)

	bucket.capacity = cap * bucket.cntPerToken
	bucket.cntGenRate = genRate * bucket.cntPerToken
	bucket.baseTime = time.Now().UnixNano()

	// Initialize the state.
	bucket.packedState.Store(sPackState(bucket.capacity, 0))

	return bucket
}

// Convert nanoseconds to milliseconds safely.
func sSafeNsToMs(ns int64) int32 {
	if ns < 0 {
		// Process underflow.
		return 0
	}

	ms := ns / 1e6
	if ms > math.MaxInt32 {
		// Process overflow.
		return math.MaxInt32
	}

	return int32(ms)
}

// Get the relative time since baseTime.
func (tb *TokenBucket) sRelTime() int32 {
	baseTime := atomic.LoadInt64(&tb.baseTime)
	return sSafeNsToMs(time.Now().UnixNano() - baseTime)
}

// Check whether it is approaching the time limit.
func (tb *TokenBucket) sCheckTimeUnsafe(currRelTime int32) bool {
	// If there is only about 12 days to reach limit.
	// Check the 31st bit of relative time.
	return (currRelTime > math.MaxInt32>>1) || currRelTime < 0
}

// Try to consume one token.
// Use lazy mode to avoid unnecessary CPU consumption
// or background rebuilds.
func (tb *TokenBucket) Allow() bool {
	// Record to avoid time exceeding loop.
	start := time.Now()

	for attempts := 0; attempts < tb.maxAttempts; attempts++ {
		// Apply exponential backoff to reduce CAS conflicts.
		if attempts >= tb.minExpBackoff {
			// Wait for over 1 microseconds.
			time.Sleep(1 << (attempts - tb.minExpBackoff) * time.Microsecond)
		}

		oldState := tb.packedState.Load()
		oldTokenCnt, oldRelTime := sUnpackState(oldState)

		// Quick check: if token is available and the time is not updated.
		// We accept millisecond precision.
		if oldTokenCnt > tb.cntPerToken && oldRelTime == tb.sRelTime() {
			newState := sPackState(oldTokenCnt-tb.cntPerToken, oldRelTime)

			if tb.packedState.CompareAndSwap(oldState, newState) {
				// Successfully consumed one token.
				return true
			}

			// If update fails, retry.
			continue
		}

		nowNs := time.Now().UnixNano()
		currRelTime := sSafeNsToMs(nowNs - tb.baseTime)
		// Handle NTP skew here.
		elapsedMs := max(int64(currRelTime-oldRelTime), 0)

		// Calculate new token count generated.We avoid float operations.
		// Handle overflow here using min.
		newTokenCnt := min(
			oldTokenCnt+uint32(elapsedMs)*tb.cntGenRate,
			tb.capacity)

		if tb.sCheckTimeUnsafe(currRelTime) {
			fmt.Println("Time overflow")

			// Time overflow, reset the state.
			newState := sPackState(newTokenCnt, 0)
			if tb.packedState.CompareAndSwap(oldState, newState) {
				// Atomicity is not guaranteed, but it is acceptable.
				atomic.StoreInt64(&tb.baseTime, nowNs)
			}

			// Try next iteration.
			continue
		}

		if newTokenCnt < tb.cntPerToken {
			// Token is not enough. So only update timestamp.
			newState := sPackState(newTokenCnt, currRelTime)

			if tb.packedState.CompareAndSwap(oldState, newState) {
				// Return false immidiately if token is not enought currently.
				return false
			}

			// If update fails, retry.
			continue
		}

		// Consume the token.
		newTokenCnt -= tb.cntPerToken
		newState := sPackState(newTokenCnt, currRelTime)

		if tb.packedState.CompareAndSwap(oldState, newState) {
			// Token is consumed successfully.
			return true
		}

		// A performance protection: if loop iterations take too long,
		// return false immidiately.
		if time.Since(start) > tb.maxAllowTime {
			return false
		}
	}

	return false
}
