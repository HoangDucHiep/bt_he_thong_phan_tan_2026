package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Role classifies a server in the fallback chain.
type Role string

const (
	RolePrimary   Role = "PRIMARY"
	RoleSecondary Role = "SECONDARY"
	RoleBackup    Role = "BACKUP"
)

// ─────────────────────────────────────────────────────────────────────────────
// QueryResult – outcome of one server query attempt
// ─────────────────────────────────────────────────────────────────────────────

type QueryResult struct {
	ServerName string
	Role       Role
	Domain     string
	IP         string
	Latency    time.Duration
	Success    bool
	ErrType    string // "TIMEOUT" | "DOWN" | "NXDOMAIN" | "RANDOM_FAIL" | ""
	ErrMsg     string
}

// ─────────────────────────────────────────────────────────────────────────────
// DNSServer
// ─────────────────────────────────────────────────────────────────────────────

type DNSServer struct {
	Name        string
	Role        Role
	records     map[string]string
	// manual kill-switch
	Down        bool
	// random failure probability [0.0, 1.0]
	FailRate    float64
	// simulated base latency (ms)
	MinLatency  int
	MaxLatency  int
	// timeout threshold (ms); query "times out" when delay exceeds this
	Timeout     int

	// per-server counters (exported for stats)
	TotalQueries   int
	Successes      int
	Failures       int
	TimeoutCount   int
	FallbackTriggered int // how many times THIS server caused a fallback
}

func NewServer(name string, role Role, minMs, maxMs, timeoutMs int, failRate float64) *DNSServer {
	return &DNSServer{
		Name:       name,
		Role:       role,
		records:    make(map[string]string),
		MinLatency: minMs,
		MaxLatency: maxMs,
		Timeout:    timeoutMs,
		FailRate:   failRate,
	}
}

func (s *DNSServer) AddRecord(domain, ip string) {
	s.records[strings.ToLower(domain)] = ip
}

// Query simulates a DNS query against this server.
func (s *DNSServer) Query(domain string) QueryResult {
	domain = strings.ToLower(domain)
	start := time.Now()

	// simulate latency
	span := s.MaxLatency - s.MinLatency
	if span < 1 {
		span = 1
	}
	delay := time.Duration(s.MinLatency+rand.Intn(span)) * time.Millisecond
	time.Sleep(delay)
	latency := time.Since(start)

	s.TotalQueries++

	res := QueryResult{
		ServerName: s.Name,
		Role:       s.Role,
		Domain:     domain,
		Latency:    latency,
	}

	// 1. Manual down
	if s.Down {
		s.Failures++
		s.FallbackTriggered++
		res.ErrType = "DOWN"
		res.ErrMsg = fmt.Sprintf("[%s] SERVER DOWN – connection refused", s.Name)
		return res
	}

	// 2. Simulated timeout (latency exceeded threshold)
	if int(latency.Milliseconds()) > s.Timeout {
		s.Failures++
		s.TimeoutCount++
		s.FallbackTriggered++
		res.ErrType = "TIMEOUT"
		res.ErrMsg = fmt.Sprintf("[%s] TIMEOUT – response took %v (limit %dms)",
			s.Name, latency.Round(time.Millisecond), s.Timeout)
		return res
	}

	// 3. Random failure
	if rand.Float64() < s.FailRate {
		s.Failures++
		s.FallbackTriggered++
		res.ErrType = "RANDOM_FAIL"
		res.ErrMsg = fmt.Sprintf("[%s] RANDOM FAILURE – server intermittently unavailable", s.Name)
		return res
	}

	// 4. Lookup
	ip, ok := s.records[domain]
	if !ok {
		s.Failures++
		res.ErrType = "NXDOMAIN"
		res.ErrMsg = fmt.Sprintf("[%s] NXDOMAIN – no record for '%s'", s.Name, domain)
		return res
	}

	s.Successes++
	res.Success = true
	res.IP = ip
	return res
}

// FailRatePct returns FailRate as a percentage string.
func (s *DNSServer) FailRatePct() string {
	return fmt.Sprintf("%.0f%%", s.FailRate*100)
}
