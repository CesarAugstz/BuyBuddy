package utils

import (
	"sync"
	"time"
)

type FirstReceiptCache struct {
	mu    sync.RWMutex
	cache map[string]*time.Time
}

type assistantSuggestionCacheEntry struct {
	suggestions []string
	expiresAt   time.Time
}

type AssistantSuggestionCache struct {
	mu    sync.RWMutex
	cache map[string]assistantSuggestionCacheEntry
}

var firstReceiptCache = &FirstReceiptCache{
	cache: make(map[string]*time.Time),
}

var assistantSuggestionCache = &AssistantSuggestionCache{
	cache: make(map[string]assistantSuggestionCacheEntry),
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

func GetAssistantSuggestionCache() *AssistantSuggestionCache {
	return assistantSuggestionCache
}

func (c *AssistantSuggestionCache) Get(userID string) ([]string, bool) {
	c.mu.RLock()
	entry, ok := c.cache[userID]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.Invalidate(userID)
		return nil, false
	}
	return append([]string(nil), entry.suggestions...), true
}

func (c *AssistantSuggestionCache) Set(userID string, suggestions []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[userID] = assistantSuggestionCacheEntry{
		suggestions: append([]string(nil), suggestions...),
		expiresAt:   time.Now().Add(ttl),
	}
}

func (c *AssistantSuggestionCache) Invalidate(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, userID)
}
