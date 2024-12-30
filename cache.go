package main

import (
	"sync"
	"time"
)

type cacheEntry struct {
	createdAt time.Time
	val       []byte
}

type Cache struct {
	mu    sync.Mutex
	store map[string]cacheEntry
}

func NewCache() *Cache {
	c := &Cache{
		store: make(map[string]cacheEntry),
	}
	go c.reapLoop(5 * time.Second) // 5 saniyede bir temizleme
	return c
}

func (c *Cache) Add(key string, val []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cacheEntry{
		createdAt: time.Now(),
		val:       val,
	}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.store[key]
	if !found {
		return nil, false
	}
	return entry.val, true
}

func (c *Cache) reapLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		for key, entry := range c.store {
			if time.Since(entry.createdAt) > interval {
				delete(c.store, key)
			}
		}
		c.mu.Unlock()
	}
}