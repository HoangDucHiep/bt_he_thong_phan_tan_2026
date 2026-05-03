package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ─────────────────────────────────────────────────────────────────────────────
// GlobalStats – session-wide counters
// ─────────────────────────────────────────────────────────────────────────────

type GlobalStats struct {
	TotalResolutions int
	Successes        int
	Failures         int
	FallbackCount    int
	PrimaryFailures  int // queries where primary was the one that failed
}

func (g *GlobalStats) PrimaryFailRate() float64 {
	if g.TotalResolutions == 0 {
		return 0
	}
	return float64(g.PrimaryFailures) / float64(g.TotalResolutions) * 100
}

// ─────────────────────────────────────────────────────────────────────────────
// FallbackResolver – tries servers in order until one succeeds
// ─────────────────────────────────────────────────────────────────────────────

type FallbackResolver struct {
	servers []*DNSServer // ordered: [primary, secondary, backup1, ...]
	stats   GlobalStats
}

func NewFallbackResolver(servers ...*DNSServer) *FallbackResolver {
	return &FallbackResolver{servers: servers}
}

// AddServer appends a server to the fallback chain.
func (r *FallbackResolver) AddServer(s *DNSServer) {
	r.servers = append(r.servers, s)
}

// RemoveServer removes a server by name. Returns true if found.
func (r *FallbackResolver) RemoveServer(name string) bool {
	for i, s := range r.servers {
		if strings.EqualFold(s.Name, name) {
			r.servers = append(r.servers[:i], r.servers[i+1:]...)
			return true
		}
	}
	return false
}

// FindServer returns the server with the given name, or nil.
func (r *FallbackResolver) FindServer(name string) *DNSServer {
	for _, s := range r.servers {
		if strings.EqualFold(s.Name, name) {
			return s
		}
	}
	return nil
}

// Servers returns all servers (read-only slice copy).
func (r *FallbackResolver) Servers() []*DNSServer {
	cp := make([]*DNSServer, len(r.servers))
	copy(cp, r.servers)
	return cp
}

// Stats returns a copy of global stats.
func (r *FallbackResolver) Stats() GlobalStats { return r.stats }

// ─────────────────────────────────────────────────────────────────────────────
// Resolve – main entry point
// ─────────────────────────────────────────────────────────────────────────────

// ResolveTrace is the full trace of one resolution attempt.
type ResolveTrace struct {
	Domain    string
	Steps     []QueryResult
	FinalIP   string
	FinalSrv  string
	Success   bool
	ErrMsg    string
	TotalTime time.Duration
}

// Resolve tries each server in order, returns a full trace.
func (r *FallbackResolver) Resolve(domain string) ResolveTrace {
	start := time.Now()
	trace := ResolveTrace{Domain: domain}

	// ── Header ────────────────────────────────────────────────────────────────
	fmt.Println()
	printBox(fmt.Sprintf("Resolving: %s", domain))
	fmt.Println()

	r.stats.TotalResolutions++
	primaryFailed := false

	for i, srv := range r.servers {
		stepNum := i + 1
		printStepHeader(stepNum, srv)

		result := srv.Query(domain)
		trace.Steps = append(trace.Steps, result)

		if result.Success {
			// ── Success ───────────────────────────────────────────────────
			color.HiGreen("         ✔  IP: %-22s  Latency: %v\n",
				result.IP, result.Latency.Round(time.Millisecond))

			trace.FinalIP = result.IP
			trace.FinalSrv = srv.Name
			trace.Success = true
			r.stats.Successes++
			if i > 0 {
				r.stats.FallbackCount++
			}
			break

		} else {
			// ── Failure ───────────────────────────────────────────────────
			errColor := color.HiRed
			prefix := "✘"
			if result.ErrType == "TIMEOUT" {
				errColor = color.HiYellow
				prefix = "⏱"
			}
			errColor("         %s  %s\n", prefix, result.ErrMsg)

			if srv.Role == RolePrimary {
				primaryFailed = true
				r.stats.PrimaryFailures++
			}

			// NXDOMAIN on any server → domain simply doesn't exist there,
			// but we still try the next server (it might have the record).
			if i < len(r.servers)-1 {
				color.HiWhite("         ↓  Falling back to next server...\n")
			}
		}
	}

	trace.TotalTime = time.Since(start)

	if !trace.Success {
		r.stats.Failures++
		trace.ErrMsg = fmt.Sprintf("All %d server(s) failed to resolve '%s'", len(r.servers), domain)
	}

	_ = primaryFailed // already counted above

	// ── Summary ───────────────────────────────────────────────────────────────
	fmt.Println()
	printResolveSummary(trace)
	return trace
}

// ─────────────────────────────────────────────────────────────────────────────
// Print helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBox(msg string) {
	w := len(msg) + 4
	border := strings.Repeat("─", w)
	color.HiWhite("┌%s┐", border)
	color.HiWhite("│  %s  │", msg)
	color.HiWhite("└%s┘", border)
}

func printStepHeader(step int, srv *DNSServer) {
	roleColor := map[Role]*color.Color{
		RolePrimary:   color.New(color.FgHiMagenta, color.Bold),
		RoleSecondary: color.New(color.FgHiCyan, color.Bold),
		RoleBackup:    color.New(color.FgHiYellow, color.Bold),
	}
	c := roleColor[srv.Role]
	if c == nil {
		c = color.New(color.FgHiWhite, color.Bold)
	}

	status := color.HiGreenString("● UP")
	if srv.Down {
		status = color.HiRedString("✘ DOWN")
	}
	failInfo := fmt.Sprintf("fail=%.0f%%  timeout=%dms  latency=%d–%dms",
		srv.FailRate*100, srv.Timeout, srv.MinLatency, srv.MaxLatency)

	c.Printf("  Step %d  [%s]  (%s)  %s  %s\n",
		step, srv.Name, srv.Role, status, color.HiBlackString(failInfo))
}

func printResolveSummary(t ResolveTrace) {
	if t.Success {
		badge := color.HiGreenString("[OK]")
		fmt.Printf("  %s  %s → %s  (via %s, total %v)\n",
			badge, t.Domain,
			color.HiWhiteString(t.FinalIP),
			color.HiCyanString(t.FinalSrv),
			t.TotalTime.Round(time.Millisecond))
	} else {
		color.HiRed("  ✘  FAILED: %s  (total %v)\n", t.ErrMsg, t.TotalTime.Round(time.Millisecond))
	}
}
