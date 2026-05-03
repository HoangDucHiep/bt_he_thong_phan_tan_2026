package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ─────────────────────────────────────────────────────────────────────────────
// StepTrace – one hop in the resolution journey
// ─────────────────────────────────────────────────────────────────────────────

type StepTrace struct {
	StepNo    int
	ServerName string
	Zone       string
	Action     string // "CACHE HIT" | "REFERRAL" | "ANSWER" | "ERROR" | "FALLBACK"
	Detail     string
	FromCache  bool
	Latency    time.Duration
}

// ResolveTrace – full trace of one resolution attempt
type ResolveTrace struct {
	Domain    string
	Steps     []StepTrace
	FinalIP   string
	FinalSrv  string
	Success   bool
	ErrMsg    string
	TotalTime time.Duration
}

// ─────────────────────────────────────────────────────────────────────────────
// GlobalStats
// ─────────────────────────────────────────────────────────────────────────────

type GlobalStats struct {
	Total     int
	Successes int
	Failures  int
	CacheHits int // hops served from cache
	Hops      int // total referral hops
	Timeouts  int
	Fallbacks int
}

// ─────────────────────────────────────────────────────────────────────────────
// DistributedResolver – iterative resolver, starts from root
// ─────────────────────────────────────────────────────────────────────────────

type DistributedResolver struct {
	root         *NameServer
	allServers   []*NameServer
	queryTimeout time.Duration // part c: per-hop timeout (applied via InternalTimeout)
	stats        GlobalStats
}

func NewDistributedResolver(root *NameServer, timeout time.Duration) *DistributedResolver {
	return &DistributedResolver{root: root, queryTimeout: timeout}
}

func (r *DistributedResolver) RegisterServer(s *NameServer) {
	r.allServers = append(r.allServers, s)
}

func (r *DistributedResolver) FindServer(name string) *NameServer {
	for _, s := range r.allServers {
		if strings.EqualFold(s.Name, name) {
			return s
		}
	}
	return nil
}

func (r *DistributedResolver) Stats() GlobalStats { return r.stats }

// Resolve performs iterative distributed name resolution starting from root.
// Each server is contacted independently — no server has global knowledge.
func (r *DistributedResolver) Resolve(domain string) ResolveTrace {
	domain = strings.ToLower(domain)
	start := time.Now()
	trace := ResolveTrace{Domain: domain}

	fmt.Println()
	printBox(fmt.Sprintf("Resolving: %s", domain))
	fmt.Println()

	r.stats.Total++
	current := r.root

	for current != nil {
		hopNo := len(trace.Steps) + 1
		printHopHeader(hopNo, current)

		reply := current.Query(domain)

		step := StepTrace{
			StepNo:     hopNo,
			ServerName: reply.ServerName,
			Zone:       current.Zone,
			Latency:    reply.Latency,
		}
		latStr := reply.Latency.Round(time.Millisecond).String()

		switch {
		case reply.Success:
			// ── Answer ──────────────────────────────────────────────────
			if reply.FromCache {
				r.stats.CacheHits++
				step.Action = "CACHE HIT"
				step.FromCache = true
				step.Detail = reply.IP
				color.HiGreen("         ✔  CACHE HIT: %-22s  (%s)\n", reply.IP, latStr)
			} else {
				step.Action = "ANSWER"
				step.Detail = reply.IP
				color.HiGreen("         ✔  ANSWER: %-22s  (%s)\n", reply.IP, latStr)
			}
			trace.Steps = append(trace.Steps, step)
			trace.FinalIP = reply.IP
			trace.FinalSrv = reply.ServerName
			trace.Success = true
			r.stats.Successes++
			current = nil

		case reply.ReferralTo != nil:
			// ── Referral ─────────────────────────────────────────────────
			r.stats.Hops++
			step.Action = "REFERRAL"
			step.Detail = "→ " + reply.ReferralTo.Name
			trace.Steps = append(trace.Steps, step)
			color.HiCyan("         ↷  REFERRAL → [%s]  zone: %s  (%s)\n",
				reply.ReferralTo.Name, reply.ReferralTo.Zone, latStr)
			current = reply.ReferralTo

		default:
			// ── Error ─────────────────────────────────────────────────────
			step.Action = "ERROR"
			step.Detail = reply.ErrMsg
			trace.Steps = append(trace.Steps, step)

			prefix, errColor := "✘", color.HiRed
			if reply.ErrType == "TIMEOUT" {
				r.stats.Timeouts++
				prefix, errColor = "⏱", color.HiYellow
			}
			errColor("         %s  %s  (%s)\n", prefix, reply.ErrMsg, latStr)

			// part c: try replica if server errored (not NXDOMAIN)
			if reply.ErrType != "NXDOMAIN" {
				fb := pickReplica(current)
				if fb != nil {
					r.stats.Fallbacks++
					color.HiCyan("         ↷  Trying replica: [%s]\n", fb.Name)
					current = fb
					continue
				}
			}
			trace.ErrMsg = reply.ErrMsg
			current = nil
		}
	}

	trace.TotalTime = time.Since(start)
	if !trace.Success {
		r.stats.Failures++
	}

	fmt.Println()
	printResolveSummary(trace)
	return trace
}

func pickReplica(srv *NameServer) *NameServer {
	for _, r := range srv.replicas {
		if !r.Down {
			return r
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Print helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBox(msg string) {
	w := len([]rune(msg)) + 4
	border := strings.Repeat("─", w)
	color.HiWhite("┌%s┐", border)
	color.HiWhite("│  %s  │", msg)
	color.HiWhite("└%s┘", border)
}

var hopColors = []*color.Color{
	color.New(color.FgHiMagenta, color.Bold),
	color.New(color.FgHiCyan, color.Bold),
	color.New(color.FgHiYellow, color.Bold),
	color.New(color.FgHiGreen, color.Bold),
	color.New(color.FgHiBlue, color.Bold),
}

func printHopHeader(step int, ns *NameServer) {
	c := hopColors[(step-1)%len(hopColors)]
	status := color.HiGreenString("● UP")
	if ns.Down {
		status = color.HiRedString("✘ DOWN")
	}
	extra := color.HiBlackString("fail=%.0f%%  lat=%d–%dms  timeout=%dms  cache=%s",
		ns.FailRate*100, ns.MinLatency, ns.MaxLatency,
		ns.InternalTimeout, ns.cache.FillBar(8))
	c.Printf("  Hop %d  [%s]  zone: %-15s  %s  %s\n",
		step, ns.Name, ns.Zone, status, extra)
}

func printResolveSummary(t ResolveTrace) {
	hops := len(t.Steps)
	if t.Success {
		fmt.Printf("  %s  %s → %s  (via %s, %d hops, %v)\n",
			color.HiGreenString("[OK]"),
			t.Domain,
			color.HiWhiteString(t.FinalIP),
			color.HiCyanString(t.FinalSrv),
			hops,
			t.TotalTime.Round(time.Millisecond))
	} else {
		color.HiRed("  ✘  FAILED: %s  (%d hops, %v)\n",
			t.ErrMsg, hops, t.TotalTime.Round(time.Millisecond))
	}
}
