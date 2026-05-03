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

	// ── Build subsystems ──────────────────────────────────────────────────────
	const cacheCapacity = 5 // small capacity → LRU eviction triggered easily

	cache := NewLRUCache(cacheCapacity)
	server := BuildDefaultServer()
	resolver := NewResolver(cache, server)

	printBanner(cacheCapacity)
	printMenu()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println()
		color.New(color.FgHiCyan, color.Bold).Print("dns-cache> ")
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

		// ── 1. resolve ────────────────────────────────────────────────────────
		case "resolve", "r":
			if len(parts) < 2 {
				color.Red("  Usage: resolve <domain>")
				continue
			}
			resolver.Resolve(parts[1])

		// ── 2. batch ──────────────────────────────────────────────────────────
		case "batch", "b":
			if len(parts) < 2 {
				color.Red("  Usage: batch <domain1> [domain2] ...")
				continue
			}
			for _, d := range parts[1:] {
				resolver.Resolve(d)
				fmt.Println(color.HiBlackString("  ──────────────────────────────────────"))
			}

		// ── 3. cache show ─────────────────────────────────────────────────────
		case "cache":
			if len(parts) < 2 {
				printCacheTable(cache)
				continue
			}
			switch strings.ToLower(parts[1]) {
			case "show", "ls", "list":
				printCacheTable(cache)
			case "flush":
				if len(parts) >= 3 {
					domain := strings.ToLower(parts[2])
					if cache.Delete(domain) {
						color.HiGreen("  ✔  Flushed '%s' from cache.\n", domain)
					} else {
						color.HiYellow("  ⚠  '%s' not found in cache.\n", domain)
					}
				} else {
					n := cache.Flush()
					color.HiGreen("  ✔  Cache flushed – %d entries removed.\n", n)
				}
			default:
				color.Red("  Unknown cache sub-command: %s", parts[1])
			}

		// ── 4. stats ──────────────────────────────────────────────────────────
		case "stats", "s":
			printStats(resolver, server)

		// ── 5. list ───────────────────────────────────────────────────────────
		case "list", "l":
			printServerRecords(server)

		// ── 6. server control ─────────────────────────────────────────────────
		case "server":
			if len(parts) < 2 {
				color.Red("  Usage: server <down|up>")
				continue
			}
			switch strings.ToLower(parts[1]) {
			case "down", "fail":
				server.Down = true
				color.HiRed("  ✘  Server [%s] is now DOWN.\n", server.Name)
			case "up", "recover":
				server.Down = false
				color.HiGreen("  ✔  Server [%s] is now UP.\n", server.Name)
			default:
				color.Red("  Unknown: server %s", parts[1])
			}

		// ── 7. config ─────────────────────────────────────────────────────────
		case "config":
			// config ttl <domain> <seconds>
			if len(parts) == 4 && strings.ToLower(parts[1]) == "ttl" {
				domain := strings.ToLower(parts[2])
				secs, err := strconv.Atoi(parts[3])
				if err != nil || secs <= 0 {
					color.Red("  Invalid seconds: %s", parts[3])
					continue
				}
				rec, ok := server.records[domain]
				if !ok {
					color.Red("  Domain '%s' not found on server.", domain)
					continue
				}
				old := rec.DefaultTTL
				rec.DefaultTTL = time.Duration(secs) * time.Second
				color.HiGreen("  ✔  TTL for '%s': %v → %v\n", domain, old, rec.DefaultTTL)
			} else {
				color.Red("  Usage: config ttl <domain> <seconds>")
			}

		// ── 8. help / menu ────────────────────────────────────────────────────
		case "help", "menu", "?", "h":
			printMenu()

		// ── 9. exit ───────────────────────────────────────────────────────────
		case "exit", "quit", "q":
			fmt.Println()
			color.HiYellow("  Final stats before exit:")
			printStats(resolver, server)
			color.Yellow("  Goodbye!")
			return

		default:
			color.Red("  Unknown command: '%s'  (type 'help' for usage)", cmd)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UI helpers
// ─────────────────────────────────────────────────────────────────────────────

func printBanner(cap int) {
	fmt.Println()
	color.Cyan("  Cache capacity : %d entries (LRU eviction when full)", cap)
	color.Cyan("  TTL range      : 15 s (short) / 30 s (medium) / 60 s (long)")
	color.Cyan("  Server timeout : ~4%% random probability per query")
	fmt.Println()
}

func printMenu() {
	fmt.Println()
	color.HiWhite("  ┌─────────────────────────────────────────────────────┐")
	color.HiWhite("  │                  AVAILABLE COMMANDS                  │")
	color.HiWhite("  ├─────────────────────────────────────────────────────┤")

	cmds := [][2]string{
		{"resolve <domain>", "Resolve domain (cache-first)"},
		{"batch <d1> [d2] ...", "Resolve multiple domains in sequence"},
		{"cache", "Show all cache entries with TTL"},
		{"cache flush", "Flush entire cache"},
		{"cache flush <domain>", "Flush a single domain from cache"},
		{"stats", "Show hit/miss/expired statistics"},
		{"list", "List all DNS records on the server"},
		{"server down", "Simulate server failure"},
		{"server up", "Recover server"},
		{"config ttl <domain> <s>", "Change TTL for a domain on server"},
		{"help", "Show this menu"},
		{"exit", "Exit (shows final stats)"},
	}
	for _, kv := range cmds {
		color.HiWhite("  │  ")
		color.HiGreen("%-28s", kv[0])
		color.HiWhite("  %s\n", kv[1])
	}
	color.HiWhite("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	color.HiYellow("  Example domains: www.example.com  mail.myorg.org  shop.viettel.vn")
	color.HiYellow("  Tip: resolve the same domain twice to see a CACHE HIT!")
}

func printCacheTable(cache *LRUCache) {
	entries := cache.Snapshot()
	fmt.Println()
	bar := cache.ProgressBar(20)
	color.HiCyan("  Cache fill: %s  (evictions so far: %d)\n", bar, cache.Evictions())
	fmt.Println()

	if len(entries) == 0 {
		color.HiWhite("  Cache is empty.\n")
		return
	}

	color.HiWhite("  %-30s  %-18s  %-10s  %-10s  %s\n",
		"Domain", "IP", "TTL Left", "Inserted", "Status")
	color.HiWhite("  %s\n", strings.Repeat("─", 90))

	for i, e := range entries {
		rem := e.RemainingTTL().Round(time.Second)
		age := time.Since(e.InsertedAt).Round(time.Second)
		status := color.HiGreenString("valid")
		if e.IsExpired() {
			status = color.HiRedString("EXPIRED")
			rem = 0
		} else if rem < 10*time.Second {
			status = color.HiYellowString("expiring")
		}
		mru := ""
		if i == 0 {
			mru = color.HiCyanString(" ← MRU")
		}
		lru := ""
		if i == len(entries)-1 && len(entries) > 1 {
			lru = color.HiMagentaString(" ← LRU")
		}
		fmt.Printf("  %-30s  %-18s  %-10s  ago:%-6s  %s%s%s\n",
			e.Domain, e.IP, rem, age, status, mru, lru)
	}
	fmt.Println()
}

func printStats(resolver *Resolver, server *DNSServer) {
	st := resolver.Stats()
	cache := resolver.Cache()
	fmt.Println()
	color.HiWhite("  ┌──────────────────────────────────────────┐")
	color.HiWhite("  │              SESSION STATISTICS           │")
	color.HiWhite("  ├──────────────────────────────────────────┤")

	rows := []struct {
		label string
		val   string
	}{
		{"Total resolutions", strconv.Itoa(st.Total())},
		{"Cache HITs       ", color.HiGreenString("%d  (%.1f%%)", st.CacheHits, st.HitRate())},
		{"Cache MISSes     ", color.HiBlueString("%d", st.CacheMisses)},
		{"Cache EXPIRED    ", color.HiYellowString("%d", st.CacheExpired)},
		{"Server errors    ", color.HiRedString("%d", st.Errors)},
		{"LRU evictions    ", color.HiMagentaString("%d", cache.Evictions())},
		{"Server queries   ", strconv.Itoa(server.TotalQueries)},
		{"Cache size       ", fmt.Sprintf("%d / %d", cache.Len(), cache.Capacity())},
	}
	for _, r := range rows {
		color.HiWhite("  │  %-20s  %s\n", r.label, r.val)
	}
	color.HiWhite("  └──────────────────────────────────────────┘")
	fmt.Println()
}

func printServerRecords(server *DNSServer) {
	records := server.ListRecords()
	sort.Slice(records, func(i, j int) bool {
		return records[i].Domain < records[j].Domain
	})
	fmt.Println()
	color.HiCyan("  DNS Server: %s\n", server.Name)
	color.HiWhite("  %-35s  %-18s  %s\n", "Domain", "IP", "Default TTL")
	color.HiWhite("  %s\n", strings.Repeat("─", 70))
	for _, r := range records {
		ttlColor := color.HiGreenString
		if r.DefaultTTL <= 15*time.Second {
			ttlColor = color.HiRedString
		} else if r.DefaultTTL <= 30*time.Second {
			ttlColor = color.HiYellowString
		}
		fmt.Printf("  %-35s  %-18s  %s\n", r.Domain, r.IP, ttlColor("%v", r.DefaultTTL))
	}
	fmt.Println()
}
