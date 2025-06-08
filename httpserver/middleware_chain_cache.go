package httpserver

import (
	"container/list"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Cache entry.
type _MiddlewareChainCacheEntry struct {
	globalMutex sync.RWMutex // finer granularity lock

	key   string
	value []MiddlewareFunc

	lastAccess atomic.Int64 // the time last time accessed
	isDirty    bool         // mark whether the entry is dirty
}

// Cache for middlewares.
// Function when a route group wants to get the full middleware chain.
// Use LRU to optimize the runtime performance.
type _MiddlewareChainCache struct {
	maxSize int // cache capacity

	rebuildInterval time.Duration // interval for rebuilding lruList
	closeChan       chan struct{} // channel for closing background goroutine

	globalMutex    sync.RWMutex           // global lock, which synchronizes read and write
	dirtyMutex     sync.Mutex             // mutex to protect dirtyList
	createMutexMap map[string]*sync.Mutex // locks at key level for creating new entry

	lruList   *list.List               // linked list for approximate LRU
	dirtyList *list.List               // linked list for rebuilding
	entries   map[string]*list.Element // store the middleware chain directly

	// for stats
	hits      atomic.Uint32
	misses    atomic.Uint32
	evictions atomic.Uint32
}

// Create a new middleware chain cache.
func _NewMiddlewareChainCache(maxSize int) *_MiddlewareChainCache {
	cache := &_MiddlewareChainCache{
		maxSize: maxSize,

		rebuildInterval: time.Second, // Rebuild every one second.
		closeChan:       make(chan struct{}),

		lruList:   list.New(),
		dirtyList: list.New(),
		entries:   make(map[string]*list.Element),

		createMutexMap: make(map[string]*sync.Mutex),
	}

	go cache._BackgroundRebuild()

	return cache
}

// Get the middleware chain.
// Create a new one if not existing in cache, and insert it into cache.
func (c *_MiddlewareChainCache) _Get(key string, creationFunc func() []MiddlewareFunc) []MiddlewareFunc {
	// Firstly, try to get the middleware chain in cache.
	if chain := c._TryGettingInCache(key); chain != nil {
		return chain
	}

	// If not, create a new one and insert it into cache.
	return c._HandleCacheMiss(key, creationFunc)
}

// Try getting a entry in cache.
// Automatically update the cache entry if successful.
func (c *_MiddlewareChainCache) _TryGettingInCache(key string) []MiddlewareFunc {
	c.globalMutex.RLock()
	// Read lock will only block a write lock aquiring.
	defer c.globalMutex.RUnlock()

	if elem, isExisting := c.entries[key]; isExisting {
		entry := elem.Value.(*_MiddlewareChainCacheEntry)

		// Update the entry for LRU.
		entry.lastAccess.Store(time.Now().UnixNano())
		// Only when it is not dirty before, block to update the dirtyList.
		func() {
			entry.globalMutex.Lock()
			defer entry.globalMutex.Unlock()

			if !entry.isDirty {
				c.dirtyMutex.Lock()
				defer c.dirtyMutex.Unlock()

				entry.isDirty = true
				c.dirtyList.PushBack(entry)
			}
		}()

		// Statistical the hits.
		c.hits.Add(1)

		return entry.value
	}

	return nil
}

// Handle when cache misses.
// Prevent cache stampede by creating a new entry ONLY ONCE in place.
func (c *_MiddlewareChainCache) _HandleCacheMiss(key string, creationFunc func() []MiddlewareFunc) []MiddlewareFunc {
	// Create a new entry.
	// Use key level lock to prevent cache stampede.
	c.globalMutex.Lock()
	keyMutex, ok := c.createMutexMap[key]
	if !ok {
		keyMutex = &sync.Mutex{}
		c.createMutexMap[key] = keyMutex
	}
	c.globalMutex.Unlock()

	// Lock to guarantee creating ONE entry.
	keyMutex.Lock()
	defer keyMutex.Unlock()

	// Double check after get lock to avoid duplicate insertion.
	if chain := c._TryGettingInCache(key); chain != nil {
		return chain
	}

	newChain := creationFunc()
	newEntry := &_MiddlewareChainCacheEntry{
		key:   key,
		value: newChain,
	}
	newEntry.lastAccess.Store(time.Now().UnixNano())

	// Get write lock to update lruList.
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	// First insertion is not viewed as dirty.
	c.lruList.PushFront(newEntry)
	c.entries[key] = c.lruList.Front()

	// Check size.Popback when over capacity.
	for c.lruList.Len() > c.maxSize {
		elem := c.lruList.Back()
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		// Delete entry from map as well.
		delete(c.entries, entry.key)
		delete(c.createMutexMap, entry.key)

		c.lruList.Remove(elem)

		c.evictions.Add(1)
	}

	c.misses.Add(1)
	return newChain
}

// Rebuild lruList in background goroutine.
func (c *_MiddlewareChainCache) _BackgroundRebuild() {
	ticker := time.NewTicker(c.rebuildInterval)
	defer ticker.Stop() // Make sure that cleanup is done.

	// Keep looping to rebuild lruList.
	for {
		select {
		case <-ticker.C:
			c._RebuildOrder()
		case <-c.closeChan:
			return
		}
	}
}

// Rebuild the order of lruList in simulate approximate LRU.
// This function will cause a long pause in processing.
func (c *_MiddlewareChainCache) _RebuildOrder() {
	// Lock write.
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	dirtyEntries := make([]*_MiddlewareChainCacheEntry, 0, c.dirtyList.Len())

	// Collect every dirty entry.
	for elem := c.dirtyList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		dirtyEntries = append(dirtyEntries, entry)
	}
	// Clear the dirty list.
	c.dirtyList.Init()

	// Sort the dirty entries by last access time, with most LRU in front.
	sort.Slice(dirtyEntries, func(i, j int) bool {
		return dirtyEntries[i].lastAccess.Load() > dirtyEntries[j].lastAccess.Load()
	})

	newLruList := list.New()
	// Record whether one entry is already in dirtyList,
	// and do not need to be pushed again when processing original lruList.
	processed := make(map[*_MiddlewareChainCacheEntry]struct{})

	// Store dirty entries first.
	for _, entry := range dirtyEntries {
		entry.isDirty = false
		newLruList.PushBack(entry)

		processed[entry] = struct{}{}
	}

	// Then store the rest entries in the front of original lruList.
	for elem := c.lruList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		if _, isProcessed := processed[entry]; isProcessed {
			continue
		}

		newLruList.PushBack(entry)
		if newLruList.Len() >= c.maxSize {
			break
		}
	}

	// Update entries mapping.
	c.entries = make(map[string]*list.Element)
	for elem := newLruList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*_MiddlewareChainCacheEntry)
		c.entries[entry.key] = elem
	}

	c.lruList = newLruList

	// log stats data
	slog.Info(fmt.Sprintf(
		"Middleware chain cache rebuild order: hits=%d, misses=%d, evictions=%d",
		c.hits.Load(), c.misses.Load(), c.evictions.Load()))
}

// Close background rebuilder goroutine.
func (c *_MiddlewareChainCache) _Close() {
	close(c.closeChan)
}
