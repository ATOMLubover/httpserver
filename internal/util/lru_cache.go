package util

import (
	"container/list"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Cache entry with generic value type.
type LruCacheEntry[T any] struct {
	// This mutex protects the whole entry content.
	rwMutex sync.RWMutex

	key   string
	value T

	// Last access time is used for LRU.
	lastAccess atomic.Int64
	// Whether the entry is used after being added into cache.
	isUsed bool
}

// Generic cache supporting different value types.
// It uses approximate LRU cache algorithm for evicting values.
type LruCache[T any] struct {
	maxSize int

	// The background goroutine will rebuild the LRU cache order every rebuildInterval.
	rebuildInterval time.Duration
	closeChan       chan struct{}

	// This mutex protects lruQue, usedQue, entries map.
	// Its write lock will be used when rebuilding cache.
	rwMutex       sync.RWMutex
	usedListMutex sync.Mutex
	entryMutexs   map[string]*sync.Mutex

	lruQue   *list.List
	usedQue  *list.List
	entryMap map[string]*list.Element

	// Exposed metrics for estimating cache hit ratio.
	metrics LruCacheMetrics
}

// Metrics of LRU cache.
type LruCacheMetrics struct {
	Hits      atomic.Uint32
	Misses    atomic.Uint32
	Evictions atomic.Uint32
}

// Create a new generic cache.
func NewLruCache[T any](maxSize int, rebuildInterval time.Duration) *LruCache[T] {
	cache := &LruCache[T]{
		maxSize: maxSize,

		rebuildInterval: rebuildInterval,
		closeChan:       make(chan struct{}),

		entryMutexs: make(map[string]*sync.Mutex),

		lruQue:   list.New(),
		usedQue:  list.New(),
		entryMap: make(map[string]*list.Element),
	}

	// Detach the background rebuild goroutine.
	go cache.sBgRebuild()

	return cache
}

// Get cached value or create if missing.
func (lc *LruCache[T]) Get(key string, newFunc func() T) T {
	if value := lc.sTryCache(key); value != nil {
		// Hit in cache.
		return *value
	}
	// Try insert missed elem into cache.
	return lc.sHandleCacheMiss(key, newFunc)
}

// Try getting a value from cache.
func (c *LruCache[T]) sTryCache(key string) *T {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	if elem, isExisting := c.entryMap[key]; isExisting {
		// Target elem is found in cache.
		entry := elem.Value.(*LruCacheEntry[T])

		// Update the entry for LRU.
		{
			entry.lastAccess.Store(time.Now().UnixNano())

			entry.rwMutex.Lock()

			if !entry.isUsed {
				c.usedListMutex.Lock()

				entry.isUsed = true
				c.usedQue.PushBack(entry)

				c.usedListMutex.Unlock()
			}

			entry.rwMutex.Unlock()
		}

		c.metrics.Hits.Add(1)
		return &entry.value
	}

	return nil
}

// Handle cache miss scenario.
func (c *LruCache[T]) sHandleCacheMiss(key string, newFunc func() T) T {
	c.rwMutex.Lock()
	keyMutex, ok := c.entryMutexs[key]
	if !ok {
		keyMutex = &sync.Mutex{}
		c.entryMutexs[key] = keyMutex
	}
	c.rwMutex.Unlock()

	keyMutex.Lock()
	defer keyMutex.Unlock()

	if value := c.sTryCache(key); value != nil {
		return *value
	}

	newValue := newFunc()
	newEntry := &LruCacheEntry[T]{
		key:   key,
		value: newValue,
	}
	newEntry.lastAccess.Store(time.Now().UnixNano())

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	c.lruQue.PushFront(newEntry)
	c.entryMap[key] = c.lruQue.Front()

	for c.lruQue.Len() > c.maxSize {
		elem := c.lruQue.Back()
		entry := elem.Value.(*LruCacheEntry[T])
		delete(c.entryMap, entry.key)
		delete(c.entryMutexs, entry.key)
		c.lruQue.Remove(elem)
		c.metrics.Evictions.Add(1)
	}

	c.metrics.Misses.Add(1)
	return newValue
}

// Background list rebuilding.
func (c *LruCache[T]) sBgRebuild() {
	ticker := time.NewTicker(c.rebuildInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sRebuildCache()
		case <-c.closeChan:
			return
		}
	}
}

// Rebuild cache order.
func (c *LruCache[T]) sRebuildCache() {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	dirtyEntries := make([]*LruCacheEntry[T], 0, c.usedQue.Len())
	for elem := c.usedQue.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*LruCacheEntry[T])
		dirtyEntries = append(dirtyEntries, entry)
	}
	c.usedQue.Init()

	sort.Slice(dirtyEntries, func(i, j int) bool {
		return dirtyEntries[i].lastAccess.Load() > dirtyEntries[j].lastAccess.Load()
	})

	newLruList := list.New()
	processed := make(map[*LruCacheEntry[T]]struct{})

	for _, entry := range dirtyEntries {
		entry.isUsed = false
		newLruList.PushBack(entry)
		processed[entry] = struct{}{}
	}

	for elem := c.lruQue.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*LruCacheEntry[T])
		if _, isProcessed := processed[entry]; isProcessed {
			continue
		}
		newLruList.PushBack(entry)
		if newLruList.Len() >= c.maxSize {
			break
		}
	}

	c.entryMap = make(map[string]*list.Element)
	for elem := newLruList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*LruCacheEntry[T])
		c.entryMap[entry.key] = elem
	}

	c.lruQue = newLruList
}

// Invalidate entries by prefix.
// Call of this function may result in long time pause in cache.
func (c *LruCache[T]) InvalidateKey(targetKey string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	toDelete := make([]*list.Element, 0)

	// Look up thru LRU queue to find entries whose key is targetKey.
	for e := c.lruQue.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*LruCacheEntry[T])
		if entry.key == targetKey {
			toDelete = append(toDelete, e)
		}
	}
	for _, e := range toDelete {
		c.lruQue.Remove(e)
	}

	// Clear the toDelete.
	toDelete = toDelete[:0]

	// Look up thru used queue to find entries whose key is targetKey.
	for e := c.usedQue.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*LruCacheEntry[T])
		if entry.key == targetKey {
			toDelete = append(toDelete, e)
		}
	}
	for _, e := range toDelete {
		c.usedQue.Remove(e)
	}

	// Finally, clear the entry map.
	for key := range c.entryMap {
		if key == targetKey {
			delete(c.entryMap, key)
		}
	}
}

// Get the metrics of the LRU cache.
func (c *LruCache[T]) Metrics() *LruCacheMetrics {
	return &c.metrics
}

// Close the background cleanup goroutine when cleanup.
func (c *LruCache[T]) Close() {
	c.closeChan <- struct{}{}
}
