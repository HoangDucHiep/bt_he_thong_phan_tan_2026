package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// CacheEntry – stored in a per-server LRU cache
// ─────────────────────────────────────────────────────────────────────────────

type CacheEntry struct {
	Domain     string
	IP         string
	TTL        time.Duration
	InsertedAt time.Time
	LastUsed   time.Time
}

func (e *CacheEntry) IsExpired() bool       { return time.Since(e.InsertedAt) > e.TTL }
func (e *CacheEntry) Remaining() time.Duration {
	r := e.TTL - time.Since(e.InsertedAt)
	if r < 0 {
		return 0
	}
	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// LRUCache – fixed-capacity LRU with TTL
// ─────────────────────────────────────────────────────────────────────────────

type LRUCache struct {
	mu       sync.Mutex
	cap      int
	items    map[string]*list.Element
	order    *list.List
	hits     int
	misses   int
	evictions int
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		cap:   capacity,
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

func (c *LRUCache) Get(domain string) (*CacheEntry, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[domain]
	if !ok {
		c.misses++
		return nil, "MISS"
	}
	e := el.Value.(*CacheEntry)
	if e.IsExpired() {
		c.order.Remove(el)
		delete(c.items, domain)
		c.misses++
		return nil, "EXPIRED"
	}
	c.order.MoveToFront(el)
	e.LastUsed = time.Now()
	c.hits++
	return e, "HIT"
}

func (c *LRUCache) Put(domain, ip string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if el, ok := c.items[domain]; ok {
		c.order.MoveToFront(el)
		e := el.Value.(*CacheEntry)
		e.IP = ip
		e.TTL = ttl
		e.InsertedAt = now
		e.LastUsed = now
		return
	}
	if c.order.Len() >= c.cap {
		lru := c.order.Back()
		if lru != nil {
			victim := lru.Value.(*CacheEntry)
			c.order.Remove(lru)
			delete(c.items, victim.Domain)
			c.evictions++
		}
	}
	e := &CacheEntry{Domain: domain, IP: ip, TTL: ttl, InsertedAt: now, LastUsed: now}
	el := c.order.PushFront(e)
	c.items[domain] = el
}

func (c *LRUCache) Snapshot() []*CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*CacheEntry, 0, c.order.Len())
	for el := c.order.Front(); el != nil; el = el.Next() {
		cp := *el.Value.(*CacheEntry)
		out = append(out, &cp)
	}
	return out
}

func (c *LRUCache) Stats() (hits, misses, evictions int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, c.evictions
}

func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

func (c *LRUCache) Flush() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.items)
	c.items = make(map[string]*list.Element)
	c.order.Init()
	return n
}

// FillBar returns an ASCII progress bar for cache fill.
func (c *LRUCache) FillBar(width int) string {
	c.mu.Lock()
	filled := c.order.Len()
	cap := c.cap
	c.mu.Unlock()
	blocks := 0
	if cap > 0 {
		blocks = int(float64(filled) / float64(cap) * float64(width))
	}
	bar := ""
	for i := 0; i < width; i++ {
		if i < blocks {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return fmt.Sprintf("[%s] %d/%d", bar, filled, cap)
}
