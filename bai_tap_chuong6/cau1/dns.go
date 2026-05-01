package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

// QueryResult represents the outcome of a single server query step.
type QueryResult struct {
	Server    string
	Layer     string
	Query     string
	Answer    string
	Latency   time.Duration
	Success   bool
	ErrMsg    string
}

// Server represents any DNS server in the hierarchy.
type Server struct {
	Name    string
	Layer   string
	records map[string]string // query → answer
	failed  bool
	// simulated base latency range (ms)
	minLatency int
	maxLatency int
	// timeout probability [0,1]
	timeoutProb float64
}

// NewServer creates a DNS server.
func NewServer(name, layer string, minMs, maxMs int, timeoutProb float64) *Server {
	return &Server{
		Name:        name,
		Layer:       layer,
		records:     make(map[string]string),
		minLatency:  minMs,
		maxLatency:  maxMs,
		timeoutProb: timeoutProb,
	}
}

// AddRecord inserts a DNS record.
func (s *Server) AddRecord(key, value string) {
	s.records[strings.ToLower(key)] = value
}

// Query simulates a DNS query to this server.
func (s *Server) Query(q string) QueryResult {
	q = strings.ToLower(q)
	start := time.Now()

	// Simulate network latency
	delay := time.Duration(s.minLatency+rand.Intn(s.maxLatency-s.minLatency+1)) * time.Millisecond
	time.Sleep(delay)
	latency := time.Since(start)

	result := QueryResult{
		Server:  s.Name,
		Layer:   s.Layer,
		Query:   q,
		Latency: latency,
	}

	// Check if server is administratively failed
	if s.failed {
		result.Success = false
		result.ErrMsg = "SERVER DOWN – connection refused"
		return result
	}

	// Random timeout simulation
	if rand.Float64() < s.timeoutProb {
		result.Success = false
		result.ErrMsg = "TIMEOUT – server did not respond in time"
		return result
	}

	// Lookup record
	if answer, ok := s.records[q]; ok {
		result.Success = true
		result.Answer = answer
	} else {
		result.Success = false
		result.ErrMsg = fmt.Sprintf("NXDOMAIN – no record for '%s'", q)
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// DNS Hierarchy
// ─────────────────────────────────────────────────────────────────────────────

// DNSHierarchy holds the full simulated DNS tree.
type DNSHierarchy struct {
	root            *Server
	tldServers      map[string]*Server // tld → server
	authServers     map[string]*Server // sld (e.g. example.com) → server
	allServers      []*Server
}

// BuildDNSHierarchy constructs and seeds the hierarchy with sample data.
func BuildDNSHierarchy() *DNSHierarchy {
	h := &DNSHierarchy{
		tldServers:  make(map[string]*Server),
		authServers: make(map[string]*Server),
	}

	// ── Root Server ─────────────────────────────────────────────────────────
	root := NewServer("root-server", "ROOT", 5, 20, 0.02)
	root.AddRecord(".com", "tld-com-server")
	root.AddRecord(".org", "tld-org-server")
	root.AddRecord(".vn",  "tld-vn-server")
	root.AddRecord(".net", "tld-net-server")
	h.root = root
	h.allServers = append(h.allServers, root)

	// ── TLD Servers ──────────────────────────────────────────────────────────
	tldCom := NewServer("tld-com-server", "TLD (.com)", 10, 40, 0.03)
	tldCom.AddRecord("example.com", "auth-example-com")
	tldCom.AddRecord("techcorp.com", "auth-techcorp-com")
	tldCom.AddRecord("store.com",    "auth-store-com")
	h.tldServers[".com"] = tldCom

	tldOrg := NewServer("tld-org-server", "TLD (.org)", 10, 40, 0.03)
	tldOrg.AddRecord("myorg.org",    "auth-myorg-org")
	tldOrg.AddRecord("opendata.org", "auth-opendata-org")
	h.tldServers[".org"] = tldOrg

	tldVn := NewServer("tld-vn-server", "TLD (.vn)", 15, 50, 0.04)
	tldVn.AddRecord("viettel.vn",  "auth-viettel-vn")
	tldVn.AddRecord("vnpay.vn",    "auth-vnpay-vn")
	h.tldServers[".vn"] = tldVn

	tldNet := NewServer("tld-net-server", "TLD (.net)", 10, 35, 0.03)
	tldNet.AddRecord("techcorp.net", "auth-techcorp-net")
	tldNet.AddRecord("cloudbase.net","auth-cloudbase-net")
	h.tldServers[".net"] = tldNet

	for _, s := range []*Server{tldCom, tldOrg, tldVn, tldNet} {
		h.allServers = append(h.allServers, s)
	}

	// ── Authoritative Servers ────────────────────────────────────────────────
	authExampleCom := NewServer("auth-example-com", "Authoritative (example.com)", 20, 60, 0.02)
	authExampleCom.AddRecord("www.example.com",   "93.184.216.34")
	authExampleCom.AddRecord("mail.example.com",  "93.184.216.35")
	authExampleCom.AddRecord("shop.example.com",  "93.184.216.36")
	authExampleCom.AddRecord("api.example.com",   "93.184.216.37")
	authExampleCom.AddRecord("news.example.com",  "93.184.216.38")
	authExampleCom.AddRecord("ftp.example.com",   "93.184.216.39")
	h.authServers["example.com"] = authExampleCom

	authTechcorpCom := NewServer("auth-techcorp-com", "Authoritative (techcorp.com)", 20, 60, 0.02)
	authTechcorpCom.AddRecord("www.techcorp.com",  "172.20.10.1")
	authTechcorpCom.AddRecord("api.techcorp.com",  "172.20.10.2")
	authTechcorpCom.AddRecord("blog.techcorp.com", "172.20.10.3")
	h.authServers["techcorp.com"] = authTechcorpCom

	authStoreCom := NewServer("auth-store-com", "Authoritative (store.com)", 20, 55, 0.05)
	authStoreCom.AddRecord("www.store.com",  "10.0.1.100")
	authStoreCom.AddRecord("shop.store.com", "10.0.1.101")
	h.authServers["store.com"] = authStoreCom

	authMyorgOrg := NewServer("auth-myorg-org", "Authoritative (myorg.org)", 20, 60, 0.02)
	authMyorgOrg.AddRecord("www.myorg.org",   "198.51.100.10")
	authMyorgOrg.AddRecord("mail.myorg.org",  "198.51.100.11")
	authMyorgOrg.AddRecord("shop.myorg.org",  "198.51.100.12")
	authMyorgOrg.AddRecord("docs.myorg.org",  "198.51.100.13")
	h.authServers["myorg.org"] = authMyorgOrg

	authOpenDataOrg := NewServer("auth-opendata-org", "Authoritative (opendata.org)", 20, 50, 0.02)
	authOpenDataOrg.AddRecord("www.opendata.org",  "203.0.113.50")
	authOpenDataOrg.AddRecord("api.opendata.org",  "203.0.113.51")
	authOpenDataOrg.AddRecord("data.opendata.org", "203.0.113.52")
	h.authServers["opendata.org"] = authOpenDataOrg

	authViettelVn := NewServer("auth-viettel-vn", "Authoritative (viettel.vn)", 25, 80, 0.03)
	authViettelVn.AddRecord("www.viettel.vn",  "118.69.88.100")
	authViettelVn.AddRecord("mail.viettel.vn", "118.69.88.101")
	authViettelVn.AddRecord("shop.viettel.vn", "118.69.88.102")
	authViettelVn.AddRecord("my.viettel.vn",   "118.69.88.103")
	h.authServers["viettel.vn"] = authViettelVn

	authVnpayVn := NewServer("auth-vnpay-vn", "Authoritative (vnpay.vn)", 25, 70, 0.03)
	authVnpayVn.AddRecord("www.vnpay.vn",  "103.9.209.10")
	authVnpayVn.AddRecord("api.vnpay.vn",  "103.9.209.11")
	authVnpayVn.AddRecord("pay.vnpay.vn",  "103.9.209.12")
	h.authServers["vnpay.vn"] = authVnpayVn

	authTechcorpNet := NewServer("auth-techcorp-net", "Authoritative (techcorp.net)", 20, 55, 0.02)
	authTechcorpNet.AddRecord("www.techcorp.net",  "172.20.20.1")
	authTechcorpNet.AddRecord("api.techcorp.net",  "172.20.20.2")
	authTechcorpNet.AddRecord("cdn.techcorp.net",  "172.20.20.3")
	h.authServers["techcorp.net"] = authTechcorpNet

	authCloudbaseNet := NewServer("auth-cloudbase-net", "Authoritative (cloudbase.net)", 15, 45, 0.02)
	authCloudbaseNet.AddRecord("www.cloudbase.net",   "192.0.2.200")
	authCloudbaseNet.AddRecord("cloud.cloudbase.net", "192.0.2.201")
	authCloudbaseNet.AddRecord("s3.cloudbase.net",    "192.0.2.202")
	h.authServers["cloudbase.net"] = authCloudbaseNet

	for _, s := range []*Server{
		authExampleCom, authTechcorpCom, authStoreCom,
		authMyorgOrg, authOpenDataOrg,
		authViettelVn, authVnpayVn,
		authTechcorpNet, authCloudbaseNet,
	} {
		h.allServers = append(h.allServers, s)
	}

	return h
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolution
// ─────────────────────────────────────────────────────────────────────────────

// Resolve performs an iterative DNS resolution for the given FQDN.
func (h *DNSHierarchy) Resolve(domain string) {
	start := time.Now()

	// ── Header ────────────────────────────────────────────────────────────────
	box := color.New(color.FgHiWhite, color.Bold)
	box.Printf("┌─────────────────────────────────────────────────────────────┐\n")
	box.Printf("│  Resolving: %-49s│\n", domain)
	box.Printf("└─────────────────────────────────────────────────────────────┘\n")
	fmt.Println()

	totalSteps := 0
	var finalIP string
	var resolved bool

	// ── Step 1: Extract TLD ────────────────────────────────────────────────────
	tld, sld, ok := parseDomain(domain)
	if !ok {
		printError("INPUT", "Invalid domain format. Expected: <host>.<sld>.<tld>")
		return
	}

	// ── Step 2: Query Root ────────────────────────────────────────────────────
	printStepHeader(1, "Root Server", "Query TLD server for '"+tld+"'")
	rootResult := h.root.Query(tld)
	totalSteps++
	printResult(rootResult)

	if !rootResult.Success {
		printFailure("Resolution failed at ROOT layer: "+rootResult.ErrMsg, time.Since(start))
		return
	}

	// ── Step 3: Query TLD ─────────────────────────────────────────────────────
	tldServer, exists := h.tldServers[tld]
	if !exists {
		printError("TLD ROUTING", "No TLD server registered for '"+tld+"'")
		return
	}

	printStepHeader(2, tldServer.Name, "Query Authoritative server for '"+sld+"'")
	tldResult := tldServer.Query(sld)
	totalSteps++
	printResult(tldResult)

	if !tldResult.Success {
		printFailure("Resolution failed at TLD layer: "+tldResult.ErrMsg, time.Since(start))
		return
	}

	// ── Step 4: Query Authoritative ───────────────────────────────────────────
	authServer, exists := h.authServers[sld]
	if !exists {
		printError("AUTH ROUTING", "No Authoritative server registered for '"+sld+"'")
		return
	}

	printStepHeader(3, authServer.Name, "Query IP for '"+domain+"'")
	authResult := authServer.Query(domain)
	totalSteps++
	printResult(authResult)

	if !authResult.Success {
		printFailure("Resolution failed at AUTHORITATIVE layer: "+authResult.ErrMsg, time.Since(start))
		return
	}

	finalIP = authResult.Answer
	resolved = true

	// ── Summary ────────────────────────────────────────────────────────────────
	total := time.Since(start)
	fmt.Println()
	if resolved {
		color.New(color.FgHiGreen, color.Bold).Printf(
			"  ✔  %s  →  %s\n", domain, finalIP)
		color.HiWhite("  Total steps: %d | Total time: %v\n", totalSteps, total.Round(time.Millisecond))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Admin
// ─────────────────────────────────────────────────────────────────────────────

// SetServerFail marks a server as failed (true) or recovered (false).
func (h *DNSHierarchy) SetServerFail(name string, fail bool) {
	name = strings.ToLower(name)
	for _, s := range h.allServers {
		if strings.EqualFold(s.Name, name) {
			s.failed = fail
			state := "RECOVERED ✔"
			c := color.New(color.FgHiGreen, color.Bold)
			if fail {
				state = "FAILED ✘"
				c = color.New(color.FgHiRed, color.Bold)
			}
			c.Printf("  Server [%s] is now %s\n", s.Name, state)
			return
		}
	}
	color.Red("  Server '%s' not found. Use 'status' to see server names.", name)
}

// PrintStatus prints all server names and their current state.
func (h *DNSHierarchy) PrintStatus() {
	fmt.Println()
	color.HiWhite("  %-40s  %-30s  %s", "Server Name", "Layer", "Status")
	color.HiWhite("  %s", strings.Repeat("─", 80))
	for _, s := range h.allServers {
		status := color.GreenString("● RUNNING")
		if s.failed {
			status = color.RedString("✘ FAILED ")
		}
		fmt.Printf("  %-40s  %-30s  %s\n", s.Name, s.Layer, status)
	}
	fmt.Println()
}

// ListDomains prints all registered domains.
func (h *DNSHierarchy) ListDomains() {
	fmt.Println()
	color.HiCyan("  All registered domains:")
	fmt.Println()

	for sld, auth := range h.authServers {
		color.HiYellow("  [%s]", sld)
		for fqdn, ip := range auth.records {
			fmt.Printf("    %-35s → %s\n", fqdn, ip)
		}
	}
	fmt.Println()
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// parseDomain splits "www.example.com" into (".com", "example.com", true).
func parseDomain(domain string) (tld, sld string, ok bool) {
	parts := strings.Split(domain, ".")
	if len(parts) < 3 {
		return "", "", false
	}
	tld = "." + parts[len(parts)-1]
	sld = parts[len(parts)-2] + tld
	return tld, sld, true
}

func printStepHeader(step int, server, action string) {
	c := layerColor(step)
	fmt.Printf("  ")
	c.Printf("Step %d", step)
	color.White(" → ")
	c.Printf("[%s]", server)
	color.White("  %s\n", action)
}

func printResult(r QueryResult) {
	latStr := fmt.Sprintf("%v", r.Latency.Round(time.Millisecond))
	if r.Success {
		color.HiGreen("         ✔  Answer: %-35s  (%s)\n", r.Answer, latStr)
	} else {
		color.HiRed("         ✘  Error: %-36s  (%s)\n", r.ErrMsg, latStr)
	}
}

func printError(stage, msg string) {
	color.HiRed("  [%s] Error: %s\n", stage, msg)
}

func printFailure(msg string, total time.Duration) {
	fmt.Println()
	color.New(color.FgHiRed, color.Bold).Printf("  ✘  Resolution FAILED: %s\n", msg)
	color.HiWhite("  Total time: %v\n", total.Round(time.Millisecond))
}

func layerColor(step int) *color.Color {
	switch step {
	case 1:
		return color.New(color.FgHiMagenta, color.Bold)
	case 2:
		return color.New(color.FgHiCyan, color.Bold)
	default:
		return color.New(color.FgHiYellow, color.Bold)
	}
}
