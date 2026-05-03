package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ─────────────────────────────────────────────────────────────────────────────
// Stats – running counters for the resolver session
// ─────────────────────────────────────────────────────────────────────────────

type Stats struct {
	CacheHits    int
	CacheMisses  int // real DNS queries that succeeded
	CacheExpired int // expired-entry misses
	Errors       int // queries that failed on the server
}

func (s *Stats) Total() int       { return s.CacheHits + s.CacheMisses + s.CacheExpired + s.Errors }
func (s *Stats) HitRate() float64 {
	total := s.Total()
	if total == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(total) * 100
}

// ─────────────────────────────────────────────────────────────────────────────
// ResolveResult – full trace of one resolution attempt
// ─────────────────────────────────────────────────────────────────────────────

type ResolveResult struct {
	Domain      string
	IP          string
	CacheStatus string        // "HIT" | "MISS" | "EXPIRED"
	TTL         time.Duration // remaining TTL (if HIT) or full TTL (if MISS)
	Latency     time.Duration // 0 for cache hits, server latency for misses
	Success     bool
	ErrMsg      string
	EvictedDomain string // set when LRU eviction occurred on Put
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolver – ties cache + server together and records stats
// ─────────────────────────────────────────────────────────────────────────────

type Resolver struct {
	cache  *LRUCache
	server *DNSServer
	stats  Stats
}

func NewResolver(cache *LRUCache, server *DNSServer) *Resolver {
	return &Resolver{cache: cache, server: server}
}

// Resolve performs a cached DNS lookup and prints a detailed trace.
func (r *Resolver) Resolve(domain string) ResolveResult {
	domain = strings.ToLower(domain)
	result := ResolveResult{Domain: domain}

	// ── Banner ────────────────────────────────────────────────────────────────
	fmt.Println()
	printBox(fmt.Sprintf("Resolving: %s", domain))
	fmt.Println()

	// ── Step 1: Check cache ───────────────────────────────────────────────────
	stepHeader(1, "Check DNS Cache")
	entry, status := r.cache.Get(domain)
	result.CacheStatus = status

	switch status {
	case "HIT":
		r.stats.CacheHits++
		result.Success = true
		result.IP = entry.IP
		result.TTL = entry.RemainingTTL()
		result.Latency = 0

		color.HiGreen("         ✔  CACHE HIT  │  IP: %-20s│  TTL remaining: %v\n",
			entry.IP, entry.RemainingTTL().Round(time.Second))
		printSummary(result)
		return result

	case "EXPIRED":
		r.stats.CacheExpired++
		color.HiYellow("         ⚠  CACHE EXPIRED – stale entry removed, querying server...\n")

	case "MISS":
		r.stats.CacheMisses++ // will undo if server fails
		color.HiWhite("         ✘  CACHE MISS – no entry found\n")
	}

	// ── Step 2: Query DNS Server ──────────────────────────────────────────────
	fmt.Println()
	stepHeader(2, fmt.Sprintf("Query DNS Server  [%s]", r.server.Name))
	srvResult := r.server.Query(domain)
	result.Latency = srvResult.Latency

	if !srvResult.Success {
		r.stats.Errors++
		if status == "MISS" {
			r.stats.CacheMisses-- // revert: it wasn't a real miss, just an error
		}
		result.ErrMsg = srvResult.ErrMsg
		color.HiRed("         ✘  Error: %-40s│ (%v)\n", srvResult.ErrMsg, srvResult.Latency.Round(time.Millisecond))
		printSummary(result)
		return result
	}

	color.HiGreen("         ✔  Answer: %-20s│  TTL: %v  │  Latency: %v\n",
		srvResult.IP,
		srvResult.TTL,
		srvResult.Latency.Round(time.Millisecond),
	)

	// ── Step 3: Store in cache ─────────────────────────────────────────────────
	fmt.Println()
	stepHeader(3, "Store in Cache")
	evicted := r.cache.Put(domain, srvResult.IP, srvResult.TTL)
	result.EvictedDomain = evicted

	if evicted != "" {
		color.HiYellow("         ⚡  LRU eviction: '%s' removed to make room\n", evicted)
	}
	color.HiCyan("         ✔  Cached: %s → %s  (TTL: %v)\n", domain, srvResult.IP, srvResult.TTL)

	result.Success = true
	result.IP = srvResult.IP
	result.TTL = srvResult.TTL
	printSummary(result)
	return result
}

// Stats returns a copy of current statistics.
func (r *Resolver) Stats() Stats { return r.stats }

// Cache exposes the underlying cache for admin operations.
func (r *Resolver) Cache() *LRUCache { return r.cache }

// Server exposes the DNS server.
func (r *Resolver) Server() *DNSServer { return r.server }

// ─────────────────────────────────────────────────────────────────────────────
// Printing helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBox(msg string) {
	w := len(msg) + 4
	border := strings.Repeat("─", w)
	color.HiWhite("┌%s┐", border)
	color.HiWhite("│  %s  │", msg)
	color.HiWhite("└%s┘", border)
}

func stepHeader(n int, label string) {
	icons := []string{"🔍", "🌐", "💾"}
	icon := ""
	if n-1 < len(icons) {
		icon = icons[n-1]
	}
	c := []*color.Color{
		color.New(color.FgHiMagenta, color.Bold),
		color.New(color.FgHiCyan, color.Bold),
		color.New(color.FgHiYellow, color.Bold),
	}[n-1]
	c.Printf("  Step %d %s  %s\n", n, icon, label)
}

func printSummary(r ResolveResult) {
	fmt.Println()
	if r.Success {
		badge := ""
		switch r.CacheStatus {
		case "HIT":
			badge = color.HiGreenString("[CACHE HIT]")
		case "MISS":
			badge = color.HiBlueString("[DNS QUERY]")
		case "EXPIRED":
			badge = color.HiYellowString("[REFRESHED]")
		}
		fmt.Printf("  %s  %s → %s\n", badge, r.Domain, color.HiWhiteString(r.IP))
		if r.CacheStatus == "HIT" {
			color.HiWhite("  Served from cache  │  TTL remaining: %v\n", r.TTL.Round(time.Second))
		} else {
			color.HiWhite("  Server latency: %v  │  TTL: %v\n",
				r.Latency.Round(time.Millisecond), r.TTL)
		}
	} else {
		color.HiRed("  ✘  Resolution FAILED: %s\n", r.ErrMsg)
	}
}
