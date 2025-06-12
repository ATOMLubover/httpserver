package httpserver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Context subpool.
// It will be created runtime when mainPool does not satisfy the requests.
type _ContextSubPool struct {
	serial int // serial number of subpool

	pool chan *Context // buffered subpool
	size int           // size of buffer

	lastAccess atomic.Int64 // number of active contexts
}

// Create a new context subpool.
func _NewContextSubPool(poolSerial int) *_ContextSubPool {
	// Subpool size will be 2 ^ (poolSerial + 1) then multiplies with the mainPool size.
	poolSize := gConfig.routerCfg.ctxPoolSize * (0x1 << (poolSerial + 1))

	p := &_ContextSubPool{
		serial: poolSerial,

		pool: make(chan *Context, poolSize),
		size: poolSize,
	}

	p.lastAccess.Store(time.Now().UnixNano())

	// Initialize contexts.
	// Prepare a slice to lessen memory fragmentation.
	ctxSlice := make([]Context, p.size)
	for i := range ctxSlice {
		tmpCtx := _NewContext()
		ctxSlice[i] = *tmpCtx

		p.pool <- &ctxSlice[i]
	}

	gLogger.Debug(fmt.Sprintf("Subpool created: %d", poolSerial))
	return p
}

// Try getting a context from the pool.
// It will not block.But it will lock the entire pool.
func (p *_ContextSubPool) _Get() (*Context, error) {
	// Try getting context without lock.
	select {
	case ctx := <-p.pool:
		return ctx, nil

	default:
		// Prevent locking.
	}

	// Return nil if the pool is empty.
	return nil, fmt.Errorf("No available context in subpool(serial: %d).", p.serial)
}

// Try returning a context back to the pool.
// Return the original cotext if failed.
func (p *_ContextSubPool) _Return(ctx *Context) *Context {
	select {
	case p.pool <- ctx:
		return nil

	default:
		// Prevent locking.
		return ctx
	}
}

// Context pool with fixed size.
// Store contexts waiting to be reused.
type _ContextPool struct {
	globalMutex sync.RWMutex // global rw mutex

	mainPool chan *Context // buffered pool to get and return contexts
	size     int           // size of buffer

	createMutexMap         map[int]*sync.Mutex // key level mutexes
	subPools               []*_ContextSubPool  // elastic subpools
	subPoolSliceSize       int                 // size of subpool slice
	subpoolCleanupInterval time.Duration       // interval to clean up unused subpools
	subpoolDropTime        time.Duration       // after what time a subpool would be dropped

	closeChan chan struct{} // channel to stop background goroutine
}

// Create a new context pool.
func _NewContextPool() *_ContextPool {
	p := &_ContextPool{
		mainPool: make(chan *Context, gConfig.routerCfg.ctxPoolSize),
		size:     gConfig.routerCfg.ctxPoolSize,

		createMutexMap:         make(map[int]*sync.Mutex, gConfig.routerCfg.ctxSubPoolNum),
		subPools:               make([]*_ContextSubPool, gConfig.routerCfg.ctxSubPoolNum),
		subPoolSliceSize:       gConfig.routerCfg.ctxSubPoolNum,
		subpoolCleanupInterval: gConfig.routerCfg.subpoolCleanupInterval,
		subpoolDropTime:        gConfig.routerCfg.subpoolDropTime,

		closeChan: make(chan struct{}),
	}

	// Initialize contexts in mainPool.
	ctxSlice := make([]Context, p.size)
	for i := range ctxSlice {
		tmpCtx := _NewContext()
		ctxSlice[i] = *tmpCtx

		p.mainPool <- &ctxSlice[i]
	}

	// Initialize key level mutexes.
	for i := range p.subPoolSliceSize {
		p.createMutexMap[i] = &sync.Mutex{}
	}

	// Run the bakcground cleanup goroutine.
	go p._BackgroundCleanup()

	return p
}

// Try getting a context from the pool.
// Will be blocked if there are no contexts available.
func (p *_ContextPool) _Get() (*Context, error) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), gConfig.routerCfg.ctxTimeoutTime)
	defer cancel()

	// First, try getting an available context from pools.
	// Search thru mainPool without lock.
	select {
	case ctx := <-p.mainPool:
		return ctx, nil

	default:
		// Prevent locking.
	}

	// If it fails, try getting from subpool.
	p.globalMutex.RLock()

	// Get a temporary snapshoot of subPoolsCopy.
	subPoolsCopy := make([]*_ContextSubPool, len(p.subPools))
	copy(subPoolsCopy, p.subPools)

	// Get empty position.
	emptyIdx := -1
	for i, subpool := range p.subPools {
		if subpool == nil {
			emptyIdx = i
			break
		}
	}

	p.globalMutex.RUnlock()

	// Search subpools without lock.
	for _, subpool := range subPoolsCopy {
		// Because sometimes there are gap between usable subpools.
		if subpool == nil {
			continue
		}

		subCtx, err := subpool._Get()
		// If context is gotton from subpools, return it.
		if err == nil {
			subpool.lastAccess.Store(time.Now().UnixNano())
			return subCtx, nil
		}
	}

	// If it fails to get a context from subpools as well,
	// try creating a new subpool if the subpools does not reach the limit.
	if emptyIdx >= 0 {
		// _CreateSubPool function ensures that the subpool is created after being finished.
		// And it returns the ref of ths subpool so we can use it without lock.
		subPool := p._CreateSubPool(emptyIdx)
		// If still too late to get a context, fallthru to search mainPool.
		if subPool != nil {
			subCtx, err := subPool._Get()
			if err == nil {
				subPool.lastAccess.Store(time.Now().UnixNano())
				return subCtx, nil
			}
		}
	}

	// Try getting a context from the mainpool again at last.
	// If no contexts available, wait for a while at mainPool,
	// which will get a context returned priorly.
	select {
	case ctx := <-p.mainPool:
		return ctx, nil

	case <-ctxTimeout.Done():
		// Return err when timeout.
		return nil, errors.New("Timeout when getting a context from pool.")
	}
}

// Return a context back to the pool.
// The context return will be automatically reset.
func (p *_ContextPool) _Return(ctx *Context) {
	// Reset before returning in case of concurrency problems.
	ctx._Reset()

	p.globalMutex.RLock()
	defer p.globalMutex.RUnlock()

	// Return ctx priorly to mainPool.
	select {
	case p.mainPool <- ctx:
		// Put the context back to the pool.
		return

	default:
		// Do not block.
	}

	// If failed to put context back to the main pool, choose a subpool to insert.
	for i := range p.subPools {
		// Try inserting to subpools.
		if p.subPools[i] != nil {
			ctx = p.subPools[i]._Return(ctx)
			// Return if successful.
			if ctx == nil {
				p.subPools[i].lastAccess.Store(time.Now().UnixNano())
				return
			}
		}
	}

	// If failed to insert to subpools, try to return to mainPool again at last,
	// and if it fails again, drop it.
	select {
	case p.mainPool <- ctx:
		// Put the context back to  mainPool.
		return

	default:
		// Drop the context.
		return
	}
}

// Try creating a subpool at the specified index.
// This function will prevent cache stampede.
// Return the index of the subpool actually created.
func (p *_ContextPool) _CreateSubPool(idx int) *_ContextSubPool {
	// Check the target subpool.
	p.globalMutex.RLock()
	if p.subPools[idx] != nil {
		p.subPools[idx].lastAccess.Store(time.Now().UnixNano())

		p.globalMutex.RUnlock()
		return p.subPools[idx]
	}

	// Find the min subpool we can create.
	actualIdx := -1
	for i := 0; i <= idx && i < p.subPoolSliceSize; i++ {
		if p.subPools[i] == nil {
			actualIdx = i
			break
		}
	}

	// If all subpools have been created, return the largest subpool.
	if actualIdx == -1 {
		// We need to update the lastAccess of the largets subpool,
		// in case that it would be deconstructed before being used.
		largest := p.subPools[p.subPoolSliceSize-1]
		largest.lastAccess.Store(time.Now().UnixNano())

		p.globalMutex.RUnlock()
		return p.subPools[p.subPoolSliceSize-1]
	}

	p.globalMutex.RUnlock()

	// Get creation lock and check whether the target subpool has been created.
	p.createMutexMap[actualIdx].Lock()
	defer p.createMutexMap[actualIdx].Unlock()

	// Double check to early return.
	p.globalMutex.RLock()
	if p.subPools[actualIdx] != nil {
		p.subPools[actualIdx].lastAccess.Store(time.Now().UnixNano())

		p.globalMutex.RUnlock()
		return p.subPools[actualIdx]
	}
	p.globalMutex.RUnlock()
	// Create subpool outside global lock.
	subPool := _NewContextSubPool(actualIdx)

	// Try to create the actual pool.
	p.globalMutex.Lock()
	p.subPools[actualIdx] = subPool
	p.subPools[actualIdx].lastAccess.Store(time.Now().UnixNano())
	p.globalMutex.Unlock()

	return p.subPools[actualIdx]
}

// Ran in background to clenaup unused subpools.
func (p *_ContextPool) _BackgroundCleanup() {
	ticker := time.NewTicker(p.subpoolCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p._Cleanup()

		case <-p.closeChan:
			return
		}
	}
}

// Cleanup unused subpools.
// This function only delete one subpool at a time.
func (p *_ContextPool) _Cleanup() {
	// Use read lock to firstly check which subpools can bee cleaned up.
	p.globalMutex.RLock()
	subpoolsCopy := p.subPools
	p.globalMutex.RUnlock()

	toDelete := -1

	// Reverse iterate to find the largest recently unused subpool.
	for i := len(subpoolsCopy) - 1; i >= 0; i-- {
		if subpoolsCopy[i] == nil {
			continue
		}

		// Check whether the subpool is unused recently.
		if time.Now().UnixNano()-subpoolsCopy[i].lastAccess.Load() > p.subpoolDropTime.Nanoseconds() {
			// Record the index of the subpool to delete.
			toDelete = i
			break
		}
	}

	if toDelete == -1 {
		return
	}

	// Delete target subpools.
	p.globalMutex.Lock()
	p.subPools[toDelete] = nil
	p.globalMutex.Unlock()

	gLogger.Debug(fmt.Sprintf("Subpool dropped: %d", toDelete))
}
