package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	resolver := buildDefaultResolver()

	printBanner()
	printMenu()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println()
		color.New(color.FgHiCyan, color.Bold).Print("dns-fallback> ")
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

		// ── resolve ─────────────────────────────────────────────────────────
		case "resolve", "r":
			if len(parts) < 2 {
				color.Red("  Usage: resolve <domain>")
				continue
			}
			resolver.Resolve(parts[1])

		// ── batch ────────────────────────────────────────────────────────────
		case "batch", "b":
			if len(parts) < 2 {
				color.Red("  Usage: batch <d1> [d2] ...")
				continue
			}
			for _, d := range parts[1:] {
				resolver.Resolve(d)
				fmt.Println(color.HiBlackString("  ──────────────────────────────────────"))
			}

		// ── status ───────────────────────────────────────────────────────────
		case "status", "st":
			printStatus(resolver)

		// ── stats ────────────────────────────────────────────────────────────
		case "stats", "s":
			printStats(resolver)

		// ── list ─────────────────────────────────────────────────────────────
		case "list", "l":
			printRecords(resolver)

		// ── server sub-commands ───────────────────────────────────────────────
		// server <name> down|up|failrate <n>|timeout <ms>|latency <min> <max>
		case "server":
			if len(parts) < 3 {
				color.Red("  Usage: server <name> <down|up|failrate <%%>|timeout <ms>|latency <min> <max>>")
				continue
			}
			name := parts[1]
			srv := resolver.FindServer(name)
			if srv == nil {
				color.Red("  Server '%s' not found. Use 'status' to list servers.", name)
				continue
			}
			subcmd := strings.ToLower(parts[2])
			switch subcmd {
			case "down", "fail":
				srv.Down = true
				color.HiRed("  ✘  [%s] is now DOWN (manual kill).\n", srv.Name)

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
					color.Red("  Invalid failrate. Use 0-100.")
					continue
				}
				old := srv.FailRate * 100
				srv.FailRate = pct / 100
				color.HiYellow("  [%s] fail rate: %.0f%% → %.0f%%\n", srv.Name, old, pct)

			case "timeout":
				if len(parts) < 4 {
					color.Red("  Usage: server <name> timeout <ms>")
					continue
				}
				ms, err := strconv.Atoi(parts[3])
				if err != nil || ms < 1 {
					color.Red("  Invalid timeout ms.")
					continue
				}
				old := srv.Timeout
				srv.Timeout = ms
				color.HiYellow("  [%s] timeout: %dms → %dms\n", srv.Name, old, ms)

			case "latency":
				if len(parts) < 5 {
					color.Red("  Usage: server <name> latency <minMs> <maxMs>")
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
				color.HiYellow("  [%s] latency: %d–%dms\n", srv.Name, mn, mx)

			default:
				color.Red("  Unknown sub-command: %s", subcmd)
			}

		// ── add backup ────────────────────────────────────────────────────────
		// addserver <name> <failrate%> <timeoutMs> <minMs> <maxMs>
		case "addserver":
			if len(parts) < 6 {
				color.Red("  Usage: addserver <name> <failrate%%> <timeoutMs> <minMs> <maxMs>")
				continue
			}
			name := parts[1]
			if resolver.FindServer(name) != nil {
				color.Red("  Server '%s' already exists.", name)
				continue
			}
			fr, e1 := strconv.ParseFloat(parts[2], 64)
			to, e2 := strconv.Atoi(parts[3])
			mn, e3 := strconv.Atoi(parts[4])
			mx, e4 := strconv.Atoi(parts[5])
			if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
				color.Red("  Invalid parameters.")
				continue
			}
			srv := NewServer(name, RoleBackup, mn, mx, to, fr/100)
			// copy all records from primary as a stub
			for d, ip := range resolver.servers[0].records {
				srv.AddRecord(d, ip)
			}
			resolver.AddServer(srv)
			color.HiGreen("  ✔  Added backup server '%s' to chain (position %d).\n",
				name, len(resolver.servers))

		// ── remove server ─────────────────────────────────────────────────────
		case "removeserver":
			if len(parts) < 2 {
				color.Red("  Usage: removeserver <name>")
				continue
			}
			if resolver.RemoveServer(parts[1]) {
				color.HiGreen("  ✔  Removed '%s' from fallback chain.\n", parts[1])
			} else {
				color.Red("  Server '%s' not found.", parts[1])
			}

		// ── help ──────────────────────────────────────────────────────────────
		case "help", "menu", "?", "h":
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
// Default resolver setup
// ─────────────────────────────────────────────────────────────────────────────

func buildDefaultResolver() *FallbackResolver {
	// dns_primary – 30% random failure, tight timeout
	primary := NewServer("dns_primary", RolePrimary, 20, 80, 100, 0.30)

	// dns_secondary – 10% failure, more generous timeout
	secondary := NewServer("dns_secondary", RoleSecondary, 30, 100, 200, 0.10)

	// dns_backup_1 – very reliable, slowest
	backup1 := NewServer("dns_backup_1", RoleBackup, 50, 150, 500, 0.03)

	// dns_backup_2 – last resort
	backup2 := NewServer("dns_backup_2", RoleBackup, 60, 180, 600, 0.01)

	// All servers share the same records (in reality they'd be synced via zone transfers).
	domains := map[string]string{
		"www.example.com":   "93.184.216.34",
		"mail.example.com":  "93.184.216.35",
		"shop.example.com":  "93.184.216.36",
		"api.example.com":   "93.184.216.37",
		"www.myorg.org":     "198.51.100.10",
		"mail.myorg.org":    "198.51.100.11",
		"shop.myorg.org":    "198.51.100.12",
		"www.viettel.vn":    "118.69.88.100",
		"mail.viettel.vn":   "118.69.88.101",
		"www.techcorp.net":  "172.20.20.1",
		"api.techcorp.net":  "172.20.20.2",
	}

	// Primary has all records
	for d, ip := range domains {
		primary.AddRecord(d, ip)
	}
	// Secondary has all records
	for d, ip := range domains {
		secondary.AddRecord(d, ip)
	}
	// backup_1 is missing some (simulates partial sync)
	for d, ip := range domains {
		if !strings.HasSuffix(d, ".vn") { // missing .vn records
			backup1.AddRecord(d, ip)
		}
	}
	// backup_2 has all records but only .com and .org
	for d, ip := range domains {
		if strings.HasSuffix(d, ".com") || strings.HasSuffix(d, ".org") {
			backup2.AddRecord(d, ip)
		}
	}

	return NewFallbackResolver(primary, secondary, backup1, backup2)
}

// ─────────────────────────────────────────────────────────────────────────────
// UI helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBanner() {
	color.Cyan("  Fallback chain: dns_primary → dns_secondary → dns_backup_1 → dns_backup_2")
	color.Cyan("  Primary fail rate: 30%%  |  Secondary: 10%%  |  Backups: 3%%, 1%%")
	fmt.Println()
}

func printMenu() {
	fmt.Println()
	color.HiWhite("  ┌───────────────────────────────────────────────────────────┐")
	color.HiWhite("  │                    AVAILABLE COMMANDS                      │")
	color.HiWhite("  ├───────────────────────────────────────────────────────────┤")
	cmds := [][2]string{
		{"resolve <domain>", "Resolve domain through fallback chain"},
		{"batch <d1> [d2] ...", "Resolve multiple domains"},
		{"status", "Show all servers & their config"},
		{"stats", "Show failure/fallback statistics"},
		{"list", "List all DNS records"},
		{"server <n> down|up", "Manually kill/recover a server (req. d)"},
		{"server <n> failrate <%%>", "Set random failure rate 0-100 (req. e)"},
		{"server <n> timeout <ms>", "Set timeout threshold in ms (req. e)"},
		{"server <n> latency <min> <max>", "Set simulated latency range (req. e)"},
		{"addserver <n> <fr%%> <to> <min> <max>", "Add backup server (req. b)"},
		{"removeserver <name>", "Remove a server from the chain"},
		{"help", "Show this menu"},
		{"exit", "Exit (shows final stats)"},
	}
	for _, kv := range cmds {
		color.HiWhite("  │  ")
		color.HiGreen("%-38s", kv[0])
		color.HiWhite("  %s\n", kv[1])
	}
	color.HiWhite("  └───────────────────────────────────────────────────────────┘")
	fmt.Println()
	color.HiYellow("  Domains: www.example.com  mail.myorg.org  www.viettel.vn  api.techcorp.net")
}

func printStatus(r *FallbackResolver) {
	fmt.Println()
	color.HiCyan("  Fallback chain order (left = highest priority):\n")
	color.HiWhite("  %-20s  %-10s  %-8s  %-8s  %-10s  %-10s  %-8s  %s\n",
		"Name", "Role", "Status", "Fail%", "Timeout", "Latency", "Queries", "Failures")
	color.HiWhite("  %s\n", strings.Repeat("─", 100))

	roleColors := map[Role]*color.Color{
		RolePrimary:   color.New(color.FgHiMagenta, color.Bold),
		RoleSecondary: color.New(color.FgHiCyan, color.Bold),
		RoleBackup:    color.New(color.FgHiYellow, color.Bold),
	}
	for i, s := range r.Servers() {
		statusStr := color.HiGreenString("● UP")
		if s.Down {
			statusStr = color.HiRedString("✘ DOWN")
		}
		rc := roleColors[s.Role]
		if rc == nil {
			rc = color.New(color.FgHiWhite)
		}
		arrow := ""
		if i < len(r.Servers())-1 {
			arrow = " →"
		}
		fmt.Printf("  ")
		rc.Printf("%-20s", s.Name)
		fmt.Printf("  %-10s  %-8s  %-8s  %-10s  %-10s  %-8d  %d%s\n",
			string(s.Role),
			statusStr,
			fmt.Sprintf("%.0f%%", s.FailRate*100),
			fmt.Sprintf("%dms", s.Timeout),
			fmt.Sprintf("%d–%dms", s.MinLatency, s.MaxLatency),
			s.TotalQueries,
			s.Failures,
			arrow,
		)
	}
	fmt.Println()
}

func printStats(r *FallbackResolver) {
	st := r.Stats()
	fmt.Println()
	color.HiWhite("  ┌──────────────────────────────────────────────┐")
	color.HiWhite("  │           FALLBACK SESSION STATISTICS         │")
	color.HiWhite("  ├──────────────────────────────────────────────┤")

	rows := []struct{ label, val string }{
		{"Total resolutions", strconv.Itoa(st.TotalResolutions)},
		{"Successes", color.HiGreenString("%d", st.Successes)},
		{"Failures (all failed)", color.HiRedString("%d", st.Failures)},
		{"Primary failures", color.HiYellowString("%d  (%.1f%% of queries)", st.PrimaryFailures, st.PrimaryFailRate())},
		{"Fallback activations", color.HiCyanString("%d", st.FallbackCount)},
	}
	for _, row := range rows {
		color.HiWhite("  │  %-22s  %s\n", row.label, row.val)
	}

	color.HiWhite("  ├──────────────────────────────────────────────┤")
	color.HiWhite("  │  Per-server breakdown:\n")
	for _, s := range r.Servers() {
		successRate := 0.0
		if s.TotalQueries > 0 {
			successRate = float64(s.Successes) / float64(s.TotalQueries) * 100
		}
		color.HiWhite("  │   %-20s  q=%-4d ok=%-4d fail=%-4d to=%-4d fallback-caused=%-4d  (%.1f%% ok)\n",
			s.Name, s.TotalQueries, s.Successes, s.Failures, s.TimeoutCount, s.FallbackTriggered, successRate)
	}
	color.HiWhite("  └──────────────────────────────────────────────┘")
	fmt.Println()
}

func printRecords(r *FallbackResolver) {
	fmt.Println()
	for _, s := range r.Servers() {
		color.HiCyan("  [%s]  (%s)\n", s.Name, s.Role)
		if len(s.records) == 0 {
			color.HiWhite("    (no records)\n")
			continue
		}
		for domain, ip := range s.records {
			fmt.Printf("    %-30s → %s\n", domain, ip)
		}
		fmt.Println()
	}
}
