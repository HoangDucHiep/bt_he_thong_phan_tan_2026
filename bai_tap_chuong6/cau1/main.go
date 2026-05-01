package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// ─── Banner ─────────────────────────────────────────────────────────────────
	printBanner()

	// ─── Build the DNS hierarchy ─────────────────────────────────────────────────
	hierarchy := BuildDNSHierarchy()

	// ─── Interactive loop ────────────────────────────────────────────────────────
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println()
		color.New(color.FgHiCyan, color.Bold).Print("DNS> ")
		fmt.Print("Enter command (resolve <domain> | status | fail <server> | recover <server> | list | help | exit): ")

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "exit", "quit":
			color.Yellow("Goodbye!")
			return

		case "resolve":
			if len(parts) < 2 {
				color.Red("Usage: resolve <domain>")
				continue
			}
			domain := strings.ToLower(parts[1])
			fmt.Println()
			hierarchy.Resolve(domain)

		case "status":
			hierarchy.PrintStatus()

		case "fail":
			if len(parts) < 2 {
				color.Red("Usage: fail <server-name>")
				continue
			}
			hierarchy.SetServerFail(parts[1], true)

		case "recover":
			if len(parts) < 2 {
				color.Red("Usage: recover <server-name>")
				continue
			}
			hierarchy.SetServerFail(parts[1], false)

		case "list":
			hierarchy.ListDomains()

		case "help":
			printHelp()

		default:
			color.Red("Unknown command: %s  (type 'help' for usage)", cmd)
		}
	}
}

func printBanner() {


	color.Cyan("  DNS Resolution Layers:")
	color.White("    Root Server    – maps TLDs (.com, .org, .vn, .net) to TLD servers")
	color.White("    TLD Servers    – maps SLDs (example.com, myorg.org) to Auth servers")
	color.White("    Auth Servers   – maps FQDNs (www.example.com) to IP addresses")
	fmt.Println()
	color.Yellow("  Type 'help' for available commands.")
}

func printHelp() {
	fmt.Println()
	color.HiWhite("  Available commands:")
	fmt.Println()
	color.Green("    resolve <domain>     ") 
	color.White("  – Resolve a domain name through the full hierarchy")
	color.Green("    list                 ")
	color.White("  – List all known domains in the system")
	color.Green("    status               ")
	color.White("  – Show status of all servers (running / failed)")
	color.Green("    fail <server>        ")
	color.White("  – Simulate a server failure")
	color.Green("    recover <server>     ")
	color.White("  – Recover a failed server")
	color.Green("    help                 ")
	color.White("  – Show this help message")
	color.Green("    exit                 ")
	color.White("  – Exit the program")
	fmt.Println()
	color.HiYellow("  Example domains you can resolve:")
	examples := []string{
		"www.example.com", "mail.example.com", "shop.example.com",
		"www.myorg.org", "mail.myorg.org", "shop.myorg.org",
		"www.techcorp.net", "api.techcorp.net",
		"www.viettel.vn", "mail.viettel.vn", "shop.viettel.vn",
		"www.opendata.org", "news.example.com",
		"unknown.example.com", "www.unknown.com",
	}
	for _, e := range examples {
		color.White("    » %s", e)
	}
	fmt.Println()
}
