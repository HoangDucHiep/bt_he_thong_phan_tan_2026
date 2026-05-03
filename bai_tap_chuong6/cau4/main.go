package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	resolver := buildSystem()

	printBanner()
	printMenu()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println()
		color.New(color.FgHiCyan, color.Bold).Print("dist-dns> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {

		// ── resolve ──────────────────────────────────────────────────────────
		case "resolve", "r":
			if len(parts) < 2 {
				color.Red("  Usage: resolve <domain>")
				continue
			}
			resolver.Resolve(parts[1])

		// ── batch ─────────────────────────────────────────────────────────────
		case "batch", "b":
			if len(parts) < 2 {
				color.Red("  Usage: batch <d1> [d2] ...")
				continue
			}
			for _, d := range parts[1:] {
				resolver.Resolve(d)
				fmt.Println(color.HiBlackString("  ──────────────────────────────────────"))
			}

		// ── status ────────────────────────────────────────────────────────────
		case "status", "st":
			printStatus(resolver)

		// ── cache ─────────────────────────────────────────────────────────────
		case "cache":
			sub := ""
			if len(parts) >= 2 {
				sub = strings.ToLower(parts[1])
			}
			switch sub {
			case "show", "ls", "":
				printAllCaches(resolver)
			case "flush":
				if len(parts) >= 3 {
					srv := resolver.FindServer(parts[2])
					if srv == nil {
						color.Red("  Server '%s' not found.", parts[2])
						continue
					}
					n := srv.cache.Flush()
					color.HiGreen("  ✔  Flushed %d entries from [%s] cache.\n", n, srv.Name)
				} else {
					total := 0
					for _, s := range resolver.allServers {
						total += s.cache.Flush()
					}
					color.HiGreen("  ✔  Flushed all caches (%d entries total).\n", total)
				}
			default:
				color.Red("  Unknown cache sub-command: %s", sub)
			}

		// ── stats ─────────────────────────────────────────────────────────────
		case "stats", "s":
			printStats(resolver)

		// ── list ──────────────────────────────────────────────────────────────
		case "list", "l":
			printRecords(resolver)

		// ── server control ────────────────────────────────────────────────────
		// server <name> down|up|failrate <n>|latency <min> <max>|timeout <ms>|ttl <s>
		case "server":
			if len(parts) < 3 {
				color.Red("  Usage: server <name> <down|up|failrate <%%>|latency <min> <max>|timeout <ms>|ttl <s>>")
				continue
			}
			srv := resolver.FindServer(parts[1])
			if srv == nil {
				color.Red("  Server '%s' not found. Use 'status'.", parts[1])
				continue
			}
			switch strings.ToLower(parts[2]) {
			case "down", "fail":
				srv.Down = true
				color.HiRed("  ✘  [%s] is now DOWN.\n", srv.Name)
			case "up", "recover":
				srv.Down = false
				color.HiGreen("  ✔  [%s] is now UP.\n", srv.Name)
			case "failrate":
				if len(parts) < 4 {
					color.Red("  Usage: server <name> failrate <0-100>")
					continue
				}
				pct, err := strconv.ParseFloat(parts[3], 64)
				if err != nil || pct < 0 || pct > 100 {
					color.Red("  Invalid value (0-100).")
					continue
				}
				srv.FailRate = pct / 100
				color.HiYellow("  [%s] fail rate → %.0f%%\n", srv.Name, pct)
			case "latency":
				if len(parts) < 5 {
					color.Red("  Usage: server <name> latency <min> <max>")
					continue
				}
				mn, e1 := strconv.Atoi(parts[3])
				mx, e2 := strconv.Atoi(parts[4])
				if e1 != nil || e2 != nil || mn < 0 || mx < mn {
					color.Red("  Invalid latency range.")
					continue
				}
				srv.MinLatency = mn
				srv.MaxLatency = mx
				color.HiYellow("  [%s] latency → %d–%dms\n", srv.Name, mn, mx)
			case "timeout":
				if len(parts) < 4 {
					color.Red("  Usage: server <name> timeout <ms>")
					continue
				}
				ms, err := strconv.Atoi(parts[3])
				if err != nil || ms < 1 {
					color.Red("  Invalid ms.")
					continue
				}
				srv.InternalTimeout = ms
				color.HiYellow("  [%s] internal timeout → %dms\n", srv.Name, ms)
			case "ttl":
				if len(parts) < 4 {
					color.Red("  Usage: server <name> ttl <seconds>")
					continue
				}
				s, err := strconv.Atoi(parts[3])
				if err != nil || s < 1 {
					color.Red("  Invalid seconds.")
					continue
				}
				srv.cacheTTL = time.Duration(s) * time.Second
				color.HiYellow("  [%s] cache TTL → %v\n", srv.Name, srv.cacheTTL)
			default:
				color.Red("  Unknown sub-command: %s", parts[2])
			}

		// ── query timeout ─────────────────────────────────────────────────────
		case "qtimeout":
			if len(parts) < 2 {
				color.Red("  Usage: qtimeout <ms>")
				continue
			}
			ms, err := strconv.Atoi(parts[1])
			if err != nil || ms < 1 {
				color.Red("  Invalid ms.")
				continue
			}
			old := resolver.queryTimeout
			resolver.queryTimeout = time.Duration(ms) * time.Millisecond
			color.HiYellow("  Query timeout: %v → %v\n", old, resolver.queryTimeout)

		// ── help ──────────────────────────────────────────────────────────────
		case "help", "h", "menu", "?":
			printMenu()

		// ── exit ──────────────────────────────────────────────────────────────
		case "exit", "quit", "q":
			fmt.Println()
			color.HiYellow("  Final stats:")
			printStats(resolver)
			color.Yellow("  Goodbye!")
			return

		default:
			color.Red("  Unknown command: '%s'  (type 'help')", cmd)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// System setup – builds the full distributed zone hierarchy
// ─────────────────────────────────────────────────────────────────────────────

func buildSystem() *DistributedResolver {
	// ── Zone: root ────────────────────────────────────────────────────────────
	root := NewNameServer("ns-root", ".", 5, 15, 50, 0.02)

	// ── Zone: com ─────────────────────────────────────────────────────────────
	nsCom := NewNameServer("ns-com", "com", 10, 30, 100, 0.03)
	// Replica for ns-com (part c)
	nsComReplica := NewNameServer("ns-com-replica", "com", 15, 40, 150, 0.01)

	// ── Zone: org ─────────────────────────────────────────────────────────────
	nsOrg := NewNameServer("ns-org", "org", 10, 30, 100, 0.03)

	// ── Zone: vn ──────────────────────────────────────────────────────────────
	nsVn := NewNameServer("ns-vn", "vn", 15, 50, 120, 0.04)

	// ── Zone: example.com ─────────────────────────────────────────────────────
	nsExampleCom := NewNameServer("ns-example-com", "example.com", 20, 60, 150, 0.02)
	nsExampleCom.AddRecord("www.example.com", "93.184.216.34")
	nsExampleCom.AddRecord("mail.example.com", "93.184.216.35")
	nsExampleCom.AddRecord("shop.example.com", "93.184.216.36")
	nsExampleCom.AddRecord("api.example.com", "93.184.216.37")
	nsExampleCom.AddRecord("ftp.example.com", "93.184.216.38")

	// ── Zone: test.com ───────────────────────────────────────────────────────
	nsTestCom := NewNameServer("ns-test-com", "test.com", 20, 60, 150, 0.02)
	nsTestCom.AddRecord("www.test.com", "203.0.113.100")
	nsTestCom.AddRecord("api.test.com", "203.0.113.101")
	nsTestCom.AddRecord("db.test.com", "203.0.113.102")

	// ── Zone: techcorp.com ────────────────────────────────────────────────────
	nsTechcorpCom := NewNameServer("ns-techcorp-com", "techcorp.com", 20, 55, 150, 0.02)
	nsTechcorpCom.AddRecord("www.techcorp.com", "172.20.10.1")
	nsTechcorpCom.AddRecord("api.techcorp.com", "172.20.10.2")
	nsTechcorpCom.AddRecord("blog.techcorp.com", "172.20.10.3")

	// ── Zone: myorg.org ───────────────────────────────────────────────────────
	nsMyorgOrg := NewNameServer("ns-myorg-org", "myorg.org", 20, 60, 150, 0.02)
	nsMyorgOrg.AddRecord("www.myorg.org", "198.51.100.10")
	nsMyorgOrg.AddRecord("mail.myorg.org", "198.51.100.11")
	nsMyorgOrg.AddRecord("shop.myorg.org", "198.51.100.12")

	// ── Zone: opendata.org ────────────────────────────────────────────────────
	nsOpendataOrg := NewNameServer("ns-opendata-org", "opendata.org", 20, 50, 150, 0.02)
	nsOpendataOrg.AddRecord("www.opendata.org", "203.0.113.50")
	nsOpendataOrg.AddRecord("api.opendata.org", "203.0.113.51")
	nsOpendataOrg.AddRecord("data.opendata.org", "203.0.113.52")

	// ── Zone: viettel.vn ──────────────────────────────────────────────────────
	nsViettelVn := NewNameServer("ns-viettel-vn", "viettel.vn", 25, 80, 200, 0.03)
	nsViettelVn.AddRecord("www.viettel.vn", "118.69.88.100")
	nsViettelVn.AddRecord("mail.viettel.vn", "118.69.88.101")
	nsViettelVn.AddRecord("my.viettel.vn", "118.69.88.102")

	// ── Wire up delegations (each server only knows its direct children) ──────
	// root → TLDs
	root.Delegate("com", nsCom)
	root.Delegate("org", nsOrg)
	root.Delegate("vn", nsVn)

	// TLD .com → SLDs
	nsCom.Delegate("example.com", nsExampleCom)
	nsCom.Delegate("test.com", nsTestCom)
	nsCom.Delegate("techcorp.com", nsTechcorpCom)
	// replica also has the same delegations
	nsComReplica.Delegate("example.com", nsExampleCom)
	nsComReplica.Delegate("test.com", nsTestCom)
	nsComReplica.Delegate("techcorp.com", nsTechcorpCom)

	// TLD .org → SLDs
	nsOrg.Delegate("myorg.org", nsMyorgOrg)
	nsOrg.Delegate("opendata.org", nsOpendataOrg)

	// TLD .vn → SLDs
	nsVn.Delegate("viettel.vn", nsViettelVn)

	// Wire replica: ns-com → ns-com-replica
	nsCom.AddReplica(nsComReplica)

	// Register all servers with the resolver
	all := []*NameServer{
		root,
		nsCom, nsComReplica,
		nsOrg, nsVn,
		nsExampleCom, nsTestCom, nsTechcorpCom,
		nsMyorgOrg, nsOpendataOrg,
		nsViettelVn,
	}
	resolver := NewDistributedResolver(root, 1*time.Second)
	for _, s := range all {
		resolver.RegisterServer(s)
	}
	return resolver
}

// ─────────────────────────────────────────────────────────────────────────────
// UI helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBanner() {
	color.Cyan("  Zone hierarchy:  root → {com, org, vn} → {example.com, test.com, ...}")
	color.Cyan("  Each server only knows its own zone + direct children (no global view)")
	color.Cyan("  Resolution: iterative referral chain  root→TLD→SLD")
	fmt.Println()
}

func printMenu() {
	fmt.Println()
	color.HiWhite("  ┌─────────────────────────────────────────────────────────────┐")
	color.HiWhite("  │                    AVAILABLE COMMANDS                         │")
	color.HiWhite("  ├─────────────────────────────────────────────────────────────┤")
	cmds := [][2]string{
		{"resolve <domain>", "Iterative distributed resolution"},
		{"batch <d1> [d2] ...", "Resolve multiple domains"},
		{"status", "Show all servers, zones, cache fill"},
		{"cache", "Show all per-server caches"},
		{"cache flush [server]", "Flush all caches or one server's cache"},
		{"stats", "Show global resolution statistics"},
		{"list", "List all DNS records across servers"},
		{"server <n> down|up", "Simulate server failure / recovery"},
		{"server <n> failrate <%>", "Set random failure rate 0–100"},
		{"server <n> latency <min> <max>", "Set simulated latency (ms)"},
		{"server <n> timeout <ms>", "Set internal timeout threshold"},
		{"server <n> ttl <s>", "Set per-server cache TTL (seconds)"},
		{"qtimeout <ms>", "Set client-side per-hop query timeout"},
		{"help", "Show this menu"},
		{"exit", "Exit (prints final stats)"},
	}
	for _, kv := range cmds {
		color.HiWhite("  │  ")
		color.HiGreen("%-36s", kv[0])
		color.HiWhite("  %s\n", kv[1])
	}
	color.HiWhite("  └─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	color.HiYellow("  Domains: www.example.com  api.test.com  mail.myorg.org  www.viettel.vn")
	color.HiYellow("           blog.techcorp.com  data.opendata.org  shop.myorg.org")
}

func printStatus(r *DistributedResolver) {
	fmt.Println()
	color.HiCyan("  Query timeout (client-side per-hop): %v\n", r.queryTimeout)
	fmt.Println()
	color.HiWhite("  %-22s  %-18s  %-8s  %-8s  %-12s  %-12s  %-8s  Cache\n",
		"Server", "Zone", "Status", "Fail%", "Timeout", "Latency", "Queries")
	color.HiWhite("  %s\n", strings.Repeat("─", 110))

	for _, s := range r.allServers {
		statusStr := color.HiGreenString("● UP  ")
		if s.Down {
			statusStr = color.HiRedString("✘ DOWN")
		}
		replicaTag := ""
		if len(s.replicas) > 0 {
			names := []string{}
			for _, rr := range s.replicas {
				names = append(names, rr.Name)
			}
			replicaTag = color.HiBlackString(" ← replicas: %s", strings.Join(names, ","))
		}
		fmt.Printf("  %-22s  %-18s  %-8s  %-8s  %-12s  %-12s  %-8d  %s%s\n",
			s.Name, s.Zone,
			statusStr,
			fmt.Sprintf("%.0f%%", s.FailRate*100),
			fmt.Sprintf("%dms", s.InternalTimeout),
			fmt.Sprintf("%d–%dms", s.MinLatency, s.MaxLatency),
			s.TotalQueries,
			s.cache.FillBar(8),
			replicaTag,
		)
	}
	fmt.Println()
}

func printAllCaches(r *DistributedResolver) {
	fmt.Println()
	hasAny := false
	for _, s := range r.allServers {
		entries := s.cache.Snapshot()
		if len(entries) == 0 {
			continue
		}
		hasAny = true
		hits, misses, evictions := s.cache.Stats()
		color.HiCyan("  [%s]  zone: %s  TTL: %v  hits=%d  misses=%d  evictions=%d\n",
			s.Name, s.Zone, s.cacheTTL, hits, misses, evictions)
		for _, e := range entries {
			expired := ""
			rem := e.Remaining()
			statusColor := color.HiGreenString
			if e.IsExpired() {
				expired = " (EXPIRED)"
				statusColor = color.HiRedString
			} else if rem < 10*time.Second {
				statusColor = color.HiYellowString
			}
			fmt.Printf("    %-30s → %-18s  TTL: %s%s\n",
				e.Domain, e.IP, statusColor("%v", rem.Round(time.Second)), expired)
		}
		fmt.Println()
	}
	if !hasAny {
		color.HiWhite("  All server caches are empty.\n")
	}
}

func printStats(r *DistributedResolver) {
	st := r.Stats()
	fmt.Println()
	color.HiWhite("  ┌──────────────────────────────────────────────────┐")
	color.HiWhite("  │       DISTRIBUTED RESOLUTION STATISTICS           │")
	color.HiWhite("  ├──────────────────────────────────────────────────┤")
	hitRate := 0.0
	if st.Total > 0 {
		hitRate = float64(st.CacheHits) / float64(st.Total) * 100
	}
	rows := []struct{ label, val string }{
		{"Total resolutions", strconv.Itoa(st.Total)},
		{"Successes", color.HiGreenString("%d", st.Successes)},
		{"Failures", color.HiRedString("%d", st.Failures)},
		{"Cache hits (across hops)", color.HiCyanString("%d  (%.1f%%)", st.CacheHits, hitRate)},
		{"Referral hops", color.HiYellowString("%d", st.Hops)},
		{"Timeouts", color.HiRedString("%d", st.Timeouts)},
		{"Fallbacks", color.HiMagentaString("%d", st.Fallbacks)},
	}
	for _, row := range rows {
		color.HiWhite("  │  %-28s  %s\n", row.label, row.val)
	}
	color.HiWhite("  ├──────────────────────────────────────────────────┤")
	color.HiWhite("  │  Per-server:\n")
	for _, s := range r.allServers {
		h, _, _ := s.cache.Stats()
		color.HiWhite("  │   %-22s  q=%-4d ok=(cache=%d) ref=%-4d fail=%-4d\n",
			s.Name, s.TotalQueries, h, s.ReferralCount, s.FailureCount)
	}
	color.HiWhite("  └──────────────────────────────────────────────────┘")
	fmt.Println()
}

func printRecords(r *DistributedResolver) {
	fmt.Println()
	servers := r.allServers
	sort.Slice(servers, func(i, j int) bool { return servers[i].Zone < servers[j].Zone })
	for _, s := range servers {
		if len(s.records) == 0 && len(s.delegations) == 0 {
			continue
		}
		color.HiCyan("  [%s]  zone: %s\n", s.Name, s.Zone)
		if len(s.delegations) > 0 {
			color.HiWhite("    Delegations:\n")
			for suffix, child := range s.delegations {
				fmt.Printf("      .%-20s → %s\n", suffix, child.Name)
			}
		}
		if len(s.records) > 0 {
			color.HiWhite("    Records:\n")
			for fqdn, ip := range s.records {
				fmt.Printf("      %-30s → %s\n", fqdn, ip)
			}
		}
		fmt.Println()
	}
}
