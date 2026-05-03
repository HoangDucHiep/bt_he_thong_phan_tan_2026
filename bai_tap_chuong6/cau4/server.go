package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// QueryReply – result of querying one NameServer
// ─────────────────────────────────────────────────────────────────────────────

type QueryReply struct {
	ServerName string
	IP         string
	ReferralTo *NameServer // follow this server next
	Success    bool
	FromCache  bool
	ErrType    string // "DOWN" | "NXDOMAIN" | "RANDOM_FAIL" | "TIMEOUT" | ""
	ErrMsg     string
	Latency    time.Duration
}

// ─────────────────────────────────────────────────────────────────────────────
// NameServer – one independent zone authority
// ─────────────────────────────────────────────────────────────────────────────

type NameServer struct {
	Name string
	Zone string // authoritative zone, e.g. ".", "com", "example.com"

	records     map[string]string    // fqdn → IP (only this zone's leaf records)
	delegations map[string]*NameServer // suffix → child NS (direct children only)

	// part b – per-server cache
	cache    *LRUCache
	cacheTTL time.Duration

	// part c – replicas for fallback
	replicas []*NameServer

	// simulated network parameters
	MinLatency      int     // ms
	MaxLatency      int     // ms
	InternalTimeout int     // ms: if simulated latency > this → report timeout
	FailRate        float64 // random failure probability [0,1]
	Down            bool    // manual kill-switch

	// per-server stats
	TotalQueries  int
	CacheHitCount int
	ReferralCount int
	FailureCount  int
}

func NewNameServer(name, zone string, minMs, maxMs, timeoutMs int, failRate float64) *NameServer {
	return &NameServer{
		Name:            name,
		Zone:            zone,
		records:         make(map[string]string),
		delegations:     make(map[string]*NameServer),
		cache:           NewLRUCache(8),
		cacheTTL:        30 * time.Second,
		MinLatency:      minMs,
		MaxLatency:      maxMs,
		InternalTimeout: timeoutMs,
		FailRate:        failRate,
	}
}

func (ns *NameServer) AddRecord(fqdn, ip string) {
	ns.records[strings.ToLower(fqdn)] = ip
}

func (ns *NameServer) Delegate(suffix string, child *NameServer) {
	ns.delegations[strings.ToLower(suffix)] = child
}

func (ns *NameServer) AddReplica(r *NameServer) {
	ns.replicas = append(ns.replicas, r)
}

// Query processes a query directly (simulates independent server processing).
// The "distributed" property is maintained by data isolation: each server
// only reads its own records/delegations, never a global table.
func (ns *NameServer) Query(domain string) QueryReply {
	domain = strings.ToLower(domain)
	start := time.Now()
	ns.TotalQueries++

	reply := QueryReply{ServerName: ns.Name}

	// simulate network latency
	span := ns.MaxLatency - ns.MinLatency
	if span < 1 {
		span = 1
	}
	delay := time.Duration(ns.MinLatency+rand.Intn(span)) * time.Millisecond
	time.Sleep(delay)
	reply.Latency = time.Since(start)

	// 1. Manual down
	if ns.Down {
		ns.FailureCount++
		reply.ErrType = "DOWN"
		reply.ErrMsg = fmt.Sprintf("[%s] SERVER DOWN – connection refused", ns.Name)
		return reply
	}

	// 2. Internal timeout (simulated latency exceeds threshold)
	if int(reply.Latency.Milliseconds()) > ns.InternalTimeout {
		ns.FailureCount++
		reply.ErrType = "TIMEOUT"
		reply.ErrMsg = fmt.Sprintf("[%s] TIMEOUT – response took %v (limit %dms)",
			ns.Name, reply.Latency.Round(time.Millisecond), ns.InternalTimeout)
		return reply
	}

	// 3. Random failure
	if rand.Float64() < ns.FailRate {
		ns.FailureCount++
		reply.ErrType = "RANDOM_FAIL"
		reply.ErrMsg = fmt.Sprintf("[%s] RANDOM FAILURE – server intermittently unavailable", ns.Name)
		return reply
	}

	// 4. Check local cache (part b)
	if entry, status := ns.cache.Get(domain); status == "HIT" {
		ns.CacheHitCount++
		reply.Success = true
		reply.IP = entry.IP
		reply.FromCache = true
		return reply
	}

	// 5. Check own authoritative records
	if ip, ok := ns.records[domain]; ok {
		ns.cache.Put(domain, ip, ns.cacheTTL)
		reply.Success = true
		reply.IP = ip
		return reply
	}

	// 6. Delegation: longest-matching suffix
	best := ""
	var bestNS *NameServer
	for suffix, child := range ns.delegations {
		if strings.HasSuffix(domain, "."+suffix) || domain == suffix {
			if len(suffix) > len(best) {
				best = suffix
				bestNS = child
			}
		}
	}
	if bestNS != nil {
		ns.ReferralCount++
		reply.ReferralTo = bestNS
		return reply
	}

	// 7. NXDOMAIN
	ns.FailureCount++
	reply.ErrType = "NXDOMAIN"
	reply.ErrMsg = fmt.Sprintf("[%s] NXDOMAIN – '%s' not in zone '%s'", ns.Name, domain, ns.Zone)
	return reply
}
