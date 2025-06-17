package util

import (
	"container/list"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cache entry with generic value type.
type LruCacheEntry[T any] struct {
	rwMutex sync.RWMutex

	key   string
	value T

	lastAccess atomic.Int64
	isUsed     bool
}

// Generic cache supporting different value types.
// It uses approximate LRU cache algorithm for evicting values.
type LruCache[T any] struct {
	maxSize int

	rebuildInterval time.Duration
	closeChan       chan struct{}

	rwMutex       sync.RWMutex
	usedListMutex sync.Mutex
	entryMutexs   map[string]*sync.Mutex

	lruQue  *list.List
	usedQue *list.List
	entries map[string]*list.Element

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

		lruQue:  list.New(),
		usedQue: list.New(),
		entries: make(map[string]*list.Element),
	}

	go cache.sBgRebld()

	return cache
}

// Get cached value or create if missing.
func (c *LruCache[T]) Get(key string, creationFunc func() T) T {
	if value := c.sTryCache(key); value != nil {
		return *value
	}
	return c.sHandleCacheMiss(key, creationFunc)
}

// Try getting a value from cache.
func (c *LruCache[T]) sTryCache(key string) *T {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	if elem, isExisting := c.entries[key]; isExisting {
		entry := elem.Value.(*LruCacheEntry[T])

		entry.lastAccess.Store(time.Now().UnixNano())
		func() {
			entry.rwMutex.Lock()
			defer entry.rwMutex.Unlock()

			if !entry.isUsed {
				c.usedListMutex.Lock()
				defer c.usedListMutex.Unlock()

				entry.isUsed = true
				c.usedQue.PushBack(entry)
			}
		}()

		c.metrics.Hits.Add(1)
		return &entry.value
	}

	return nil
}

// Handle cache miss scenario.
func (c *LruCache[T]) sHandleCacheMiss(key string, creationFunc func() T) T {
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

	newValue := creationFunc()
	newEntry := &LruCacheEntry[T]{
		key:   key,
		value: newValue,
	}
	newEntry.lastAccess.Store(time.Now().UnixNano())

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	c.lruQue.PushFront(newEntry)
	c.entries[key] = c.lruQue.Front()

	for c.lruQue.Len() > c.maxSize {
		elem := c.lruQue.Back()
		entry := elem.Value.(*LruCacheEntry[T])
		delete(c.entries, entry.key)
		delete(c.entryMutexs, entry.key)
		c.lruQue.Remove(elem)
		c.metrics.Evictions.Add(1)
	}

	c.metrics.Misses.Add(1)
	return newValue
}

// Background list rebuilding.
func (c *LruCache[T]) sBgRebld() {
	ticker := time.NewTicker(c.rebuildInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sRebld()
		case <-c.closeChan:
			return
		}
	}
}

// Rebuild cache order.
func (c *LruCache[T]) sRebld() {
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

	c.entries = make(map[string]*list.Element)
	for elem := newLruList.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*LruCacheEntry[T])
		c.entries[entry.key] = elem
	}

	c.lruQue = newLruList
}

// Invalidate entries by prefix.
func (c *LruCache[T]) InvalidateKey(key string) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	toDelete := make([]*list.Element, 0)

	// Look up thru LRU queue to find entries whose prefix is key.
	for e := c.lruQue.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*LruCacheEntry[T])
		if strings.HasPrefix(entry.key, key) {
			toDelete = append(toDelete, e)
		}
	}
	for _, e := range toDelete {
		c.lruQue.Remove(e)
	}

	// Clear the toDelete.
	toDelete = toDelete[:0]

	// Look up thru used queue to find entries whose prefix is key.
	for e := c.usedQue.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*LruCacheEntry[T])
		if strings.HasPrefix(entry.key, key) {
			toDelete = append(toDelete, e)
		}
	}
	for _, e := range toDelete {
		c.usedQue.Remove(e)
	}

	// Finally, clear the entry map.
	for k := range c.entries {
		if strings.HasPrefix(k, key) {
			delete(c.entries, k)
		}
	}
}

// Get the metrics of the LRU cache.
func (c *LruCache[T]) Metrics() *LruCacheMetrics {
	return &c.metrics
}
