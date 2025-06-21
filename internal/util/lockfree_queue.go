package util

import (
	"runtime"
	"sync/atomic"
)

// Calculate the next power of two which is just larger than n.
func sNextPowerOfTwo(n int64) int64 {
	if n <= 0 {
		return 1
	}

	// Set all lower bits to 1.
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32

	// Add 1 to make n a power of two.
	n++

	return n
}

const ( // The size of a cache line in bytes.
	kCacheLineSize = 64

	// Max retry time when spinning.
	kDefaultMaxSpinTime = 50
	// Producer goscheduler counter.
	kDefaultProdGoschedMask = 0xF
	// Consumer goscheduler counter.
	kDefaultConsGoschedMask = 0xF
)

// A cell is a single entry in a lock-free queue.
type sCell[T any] struct {
	// The object stored.
	data T

	// The sequence number of the cell,
	// coordinating the producers and consumers.
	sequence atomic.Int64

	// _ [kCacheLineSize]byte
}

// Con-currency safe lock-free queue.
// It has fixed size.This implementation is used for MPMC.
// The algorithm used is based on Michael-Scott's algorithm.
type MpmcLockfreeQueue[T any] struct {
	// Prevent false sharing.
	_ [kCacheLineSize]byte

	// Capacity of the queue.
	// It must be the power of 2 to imporve performance.
	capacity int64

	// The index of the consumer.
	head atomic.Int64
	_    [kCacheLineSize]byte

	// The index of the producer.
	tail atomic.Int64
	_    [kCacheLineSize]byte

	// Ring buffer of cells with fixed size.
	ringBuf []sCell[T]
}

// Create a new lock-free queue.
// Promises that the capacity will not smaller than cap.
func NewMpmcLockfreeQueue[T any](cap int64) *MpmcLockfreeQueue[T] {
	// Get the actual capacity.
	cap = sNextPowerOfTwo(cap)

	// Initialize the ring buffer.
	buf := make([]sCell[T], cap)
	for i := range buf {
		buf[i].sequence.Store(int64(i))
	}

	return &MpmcLockfreeQueue[T]{
		capacity: cap,

		ringBuf: buf,

		head: atomic.Int64{},
		tail: atomic.Int64{},
	}
}

// Push a new item into the queue at the tail.
// Returns false if not successful(queue is full, etc).
func (lq *MpmcLockfreeQueue[T]) PushBack(data T) bool {
	// Use loop + CAS to implement lock-free.
	for i := range kDefaultMaxSpinTime {
		// Get the current tail.
		tail := lq.tail.Load()
		index := tail & (lq.capacity - 1)

		cell := &lq.ringBuf[index]
		seq := cell.sequence.Load()

		// Get the status of specific cell.
		diff := seq - tail

		switch {
		case diff == 0:
			// If this slot is empty.
			if lq.tail.CompareAndSwap(tail, tail+1) {
				// Secure the slot successfully.

				// Copy the data from stack to heap.
				cell.data = data
				cell.sequence.Store(tail + 1)

				return true
			}

		case diff < 0:
			// Queue is full.
			return false

		default:
			// The current slot is occupied.
			if i&kDefaultProdGoschedMask == 0 {
				// Sping every 32 iteration, yield to other goroutines.
				runtime.Gosched()
			}
		}
	}

	// If failed over kMaxSpinTime, return false.
	return false
}

// PopFront the elem from the queue.
// Returns (default value of T), false if the queue is empty.
func (lq *MpmcLockfreeQueue[T]) PopFront() (result T, ok bool) {
	for i := range kDefaultMaxSpinTime {
		head := lq.head.Load()
		index := head & (lq.capacity - 1)

		cell := &lq.ringBuf[index]
		seq := cell.sequence.Load()

		// (head + 1) is the expected sequence number.
		diff := seq - (head + 1)

		switch {
		case diff == 0:
			// The slot is available.
			if lq.head.CompareAndSwap(head, head+1) {
				// Take the slot successfully.
				// This slot is marked taken and unavailable.

				// We must copy the data to avoid race condition,
				// which is possible between Store and return
				// (because after Store the slot will be able to be reused,
				// but at return we read it again)
				result = cell.data
				ok = true
				cell.sequence.Store(head + lq.capacity)

				return
			}

		case diff < 0:
			// Queue is empty.
			var zero T
			return zero, false

		default:
			// The current slot is not available.
			// Try again.
			if i&kDefaultConsGoschedMask == 0 {
				// Sping every 32 iteration, yield to other goroutines.
				runtime.Gosched()
			}
		}
	}

	result = *new(T)
	ok = false
	return
}

// Get the capacity of queue.
func (lq *MpmcLockfreeQueue[T]) Capacity() int64 {
	return lq.capacity
}
