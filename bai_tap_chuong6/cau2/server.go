package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// DNSRecord – one record on the authoritative server
// ─────────────────────────────────────────────────────────────────────────────

type DNSRecord struct {
	Domain     string
	IP         string
	DefaultTTL time.Duration // TTL advertised by the server
}

// ─────────────────────────────────────────────────────────────────────────────
// DNSServer – simulated single authoritative name server
// ─────────────────────────────────────────────────────────────────────────────

type DNSServer struct {
	Name       string
	records    map[string]*DNSRecord // domain (lower) → record
	minLatency time.Duration         // simulated network delay range
	maxLatency time.Duration
	// probability [0,1] of random timeout
	timeoutProb float64
	// if true every query fails with "connection refused"
	Down bool
	// counters
	TotalQueries int
}

func NewDNSServer(name string, minMs, maxMs int, timeoutProb float64) *DNSServer {
	return &DNSServer{
		Name:        name,
		records:     make(map[string]*DNSRecord),
		minLatency:  time.Duration(minMs) * time.Millisecond,
		maxLatency:  time.Duration(maxMs) * time.Millisecond,
		timeoutProb: timeoutProb,
	}
}

func (s *DNSServer) AddRecord(domain, ip string, ttl time.Duration) {
	s.records[strings.ToLower(domain)] = &DNSRecord{
		Domain:     strings.ToLower(domain),
		IP:         ip,
		DefaultTTL: ttl,
	}
}

// ServerQueryResult is the raw outcome of hitting the DNS server.
type ServerQueryResult struct {
	Domain  string
	IP      string
	TTL     time.Duration
	Latency time.Duration
	Success bool
	ErrMsg  string
}

// Query simulates a network lookup against this server.
func (s *DNSServer) Query(domain string) ServerQueryResult {
	domain = strings.ToLower(domain)
	start := time.Now()

	// simulate latency
	span := s.maxLatency - s.minLatency
	jitter := time.Duration(rand.Int63n(int64(span) + 1))
	time.Sleep(s.minLatency + jitter)

	latency := time.Since(start)
	res := ServerQueryResult{Domain: domain, Latency: latency}

	s.TotalQueries++

	if s.Down {
		res.ErrMsg = "SERVER DOWN – connection refused"
		return res
	}
	if rand.Float64() < s.timeoutProb {
		res.ErrMsg = fmt.Sprintf("TIMEOUT – no response from %s", s.Name)
		return res
	}

	rec, ok := s.records[domain]
	if !ok {
		res.ErrMsg = fmt.Sprintf("NXDOMAIN – no record for '%s'", domain)
		return res
	}

	res.Success = true
	res.IP = rec.IP
	res.TTL = rec.DefaultTTL
	return res
}

// ListRecords returns all registered records.
func (s *DNSServer) ListRecords() []*DNSRecord {
	out := make([]*DNSRecord, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, r)
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Build the default server with a rich record set
// ─────────────────────────────────────────────────────────────────────────────

func BuildDefaultServer() *DNSServer {
	srv := NewDNSServer("authoritative-dns-server", 30, 120, 0.04)

	// short TTL – expires quickly (good for demo)
	short := 15 * time.Second
	// medium TTL
	med := 30 * time.Second
	// long TTL
	long := 60 * time.Second

	records := []struct {
		domain string
		ip     string
		ttl    time.Duration
	}{
		// example.com zone
		{"www.example.com", "93.184.216.34", med},
		{"mail.example.com", "93.184.216.35", long},
		{"shop.example.com", "93.184.216.36", short},
		{"api.example.com", "93.184.216.37", med},
		{"news.example.com", "93.184.216.38", short},
		{"ftp.example.com", "93.184.216.39", long},

		// myorg.org zone
		{"www.myorg.org", "198.51.100.10", med},
		{"mail.myorg.org", "198.51.100.11", long},
		{"shop.myorg.org", "198.51.100.12", short},
		{"docs.myorg.org", "198.51.100.13", med},

		// viettel.vn zone
		{"www.viettel.vn", "118.69.88.100", long},
		{"mail.viettel.vn", "118.69.88.101", med},
		{"shop.viettel.vn", "118.69.88.102", short},
		{"my.viettel.vn", "118.69.88.103", med},

		// techcorp.net zone
		{"www.techcorp.net", "172.20.20.1", long},
		{"api.techcorp.net", "172.20.20.2", med},
		{"cdn.techcorp.net", "172.20.20.3", short},

		// opendata.org zone
		{"www.opendata.org", "203.0.113.50", med},
		{"api.opendata.org", "203.0.113.51", short},
		{"data.opendata.org", "203.0.113.52", long},
	}

	for _, r := range records {
		srv.AddRecord(r.domain, r.ip, r.ttl)
	}
	return srv
}
