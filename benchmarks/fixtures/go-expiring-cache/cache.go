package expiringcache

import (
	"sync"
	"time"
)

type entry struct {
	value     string
	expiresAt time.Time
}

type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

func New() *Cache {
	return newWithClock(time.Now)
}

func newWithClock(now func() time.Time) *Cache {
	return &Cache{
		entries: make(map[string]entry),
		now:     now,
	}
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry{
		value:     value,
		expiresAt: c.now().Add(ttl),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, found := c.entries[key]
	if !found {
		return "", false
	}
	return item.value, true
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
