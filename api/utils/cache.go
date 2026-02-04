package utils

import (
	"sync"
	"time"
)

type FirstReceiptCache struct {
	mu    sync.RWMutex
	cache map[string]*time.Time
}

var firstReceiptCache = &FirstReceiptCache{
	cache: make(map[string]*time.Time),
}

func GetFirstReceiptCache() *FirstReceiptCache {
	return firstReceiptCache
}

func (c *FirstReceiptCache) Get(userID string) (*time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	date, ok := c.cache[userID]
	return date, ok
}

func (c *FirstReceiptCache) Set(userID string, date *time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[userID] = date
}

func (c *FirstReceiptCache) Invalidate(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, userID)
}
