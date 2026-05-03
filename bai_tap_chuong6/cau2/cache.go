package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// CacheEntry – one record stored in the LRU cache
// ─────────────────────────────────────────────────────────────────────────────

type CacheEntry struct {
	Domain    string
	IP        string
	TTL       time.Duration // configured TTL for this record
	InsertedAt time.Time    // when the entry was added / last refreshed
	LastUsed  time.Time    // for LRU ordering (updated on every hit)
}

// RemainingTTL returns how much time is left before this entry expires.
func (e *CacheEntry) RemainingTTL() time.Duration {
	elapsed := time.Since(e.InsertedAt)
	rem := e.TTL - elapsed
	if rem < 0 {
		return 0
	}
	return rem
}

// IsExpired returns true when the TTL has elapsed.
func (e *CacheEntry) IsExpired() bool {
	return time.Since(e.InsertedAt) > e.TTL
}

// ─────────────────────────────────────────────────────────────────────────────
// LRUCache – fixed-capacity LRU cache with per-entry TTL
// ─────────────────────────────────────────────────────────────────────────────

type LRUCache struct {
	mu       sync.Mutex
	cap      int                      // maximum number of entries
	items    map[string]*list.Element // domain → list element
	order    *list.List               // front = most recently used
	evictions int                     // total LRU evictions
}

// NewLRUCache creates an LRU cache with the given capacity.
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		cap:   capacity,
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Get retrieves an entry. Returns (entry, "HIT"), (nil, "EXPIRED"), or (nil, "MISS").
func (c *LRUCache) Get(domain string) (*CacheEntry, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[domain]
	if !ok {
		return nil, "MISS"
	}
	entry := el.Value.(*CacheEntry)
	if entry.IsExpired() {
		// remove stale entry
		c.order.Remove(el)
		delete(c.items, domain)
		return nil, "EXPIRED"
	}
	// move to front (most recently used)
	c.order.MoveToFront(el)
	entry.LastUsed = time.Now()
	return entry, "HIT"
}

// Put inserts or updates an entry. If at capacity, evicts the LRU element.
// Returns the name of the evicted domain (or "").
func (c *LRUCache) Put(domain, ip string, ttl time.Duration) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// update existing
	if el, ok := c.items[domain]; ok {
		c.order.MoveToFront(el)
		entry := el.Value.(*CacheEntry)
		entry.IP = ip
		entry.TTL = ttl
		entry.InsertedAt = now
		entry.LastUsed = now
		return ""
	}

	// evict LRU if at capacity
	evicted := ""
	if c.order.Len() >= c.cap {
		lru := c.order.Back()
		if lru != nil {
			victim := lru.Value.(*CacheEntry)
			evicted = victim.Domain
			c.order.Remove(lru)
			delete(c.items, victim.Domain)
			c.evictions++
		}
	}

	// insert new entry at front
	entry := &CacheEntry{
		Domain:     domain,
		IP:         ip,
		TTL:        ttl,
		InsertedAt: now,
		LastUsed:   now,
	}
	el := c.order.PushFront(entry)
	c.items[domain] = el
	return evicted
}

// Delete removes a single entry. Returns true if it existed.
func (c *LRUCache) Delete(domain string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[domain]
	if !ok {
		return false
	}
	c.order.Remove(el)
	delete(c.items, domain)
	return true
}

// Flush removes all entries and returns the count removed.
func (c *LRUCache) Flush() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.items)
	c.items = make(map[string]*list.Element)
	c.order.Init()
	return n
}

// Snapshot returns all entries ordered from MRU to LRU (defensive copy).
func (c *LRUCache) Snapshot() []*CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*CacheEntry, 0, c.order.Len())
	for el := c.order.Front(); el != nil; el = el.Next() {
		e := el.Value.(*CacheEntry)
		cp := *e // copy so callers can't mutate
		out = append(out, &cp)
	}
	return out
}

// Len returns the current number of entries (including possibly expired ones
// that haven't been evicted yet).
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Capacity returns the maximum allowed entries.
func (c *LRUCache) Capacity() int { return c.cap }

// Evictions returns total LRU evictions since creation.
func (c *LRUCache) Evictions() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictions
}

// ProgressBar renders a simple ASCII bar for cache fill level.
func (c *LRUCache) ProgressBar(width int) string {
	c.mu.Lock()
	filled := c.order.Len()
	cap := c.cap
	c.mu.Unlock()

	ratio := float64(filled) / float64(cap)
	blocks := int(ratio * float64(width))
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
