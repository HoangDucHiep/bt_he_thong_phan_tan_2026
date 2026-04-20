// Vector Clock — Interactive TCP version
//
// Each process maintains a vector of logical clocks, one entry per process.
// Rules:
//   Internal event  : V[self]++
//   Send message    : V[self]++, attach full vector V to message
//   Receive message : V[j] = max(V[j], msg.V[j]) for all j, then V[self]++
//
// Usage (each in its own terminal):
//   go run main.go <my-id> <peer-id> [peer-id ...]
//
// Example (3 processes, IDs 1 2 3):
//   Terminal 1:  go run main.go 1 2 3
//   Terminal 2:  go run main.go 2 1 3
//   Terminal 3:  go run main.go 3 1 2
//
// Commands:
//   i                    — internal event  (V[self]++)
//   send <id> [text]     — send message to process <id>
//   s    <id> [text]     — alias for send
//   cmp  <v1> <v2>       — compare two snapshot labels (from log) -- see below
//   q                    — quit
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Vector clock helpers
// ---------------------------------------------------------------------------

// VClock maps processID → logical time.
type VClock map[int]int

// copy returns a deep copy.
func (v VClock) copy() VClock {
	c := make(VClock, len(v))
	for k, val := range v {
		c[k] = val
	}
	return c
}

// merge applies component-wise max in-place: v[k] = max(v[k], other[k]).
func (v VClock) merge(other VClock) {
	for k, val := range other {
		if val > v[k] {
			v[k] = val
		}
	}
}

// format prints the vector in sorted-ID order: [1:0 2:3 3:1]
func (v VClock) format(allIDs []int) string {
	parts := make([]string, 0, len(allIDs))
	for _, id := range allIDs {
		parts = append(parts, fmt.Sprintf("P%d:%d", id, v[id]))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// Causality relations
func happensBefore(a, b VClock) bool {
	// a → b  iff  a[k] ≤ b[k] for all k  AND  a[k] < b[k] for some k
	oneStrict := false
	for k, av := range a {
		bv := b[k]
		if av > bv {
			return false
		}
		if av < bv {
			oneStrict = true
		}
	}
	return oneStrict
}

func concurrent(a, b VClock) bool {
	return !happensBefore(a, b) && !happensBefore(b, a)
}

// ---------------------------------------------------------------------------
// Message
// ---------------------------------------------------------------------------

type Message struct {
	From        int               `json:"from"`
	Vector      VClock            `json:"vector"`       // sender's vector at send time
	Text        string            `json:"text"`
	SenderLabel string            `json:"sender_label"` // label of the send-event on the sender
	History     map[string]VClock `json:"history"`      // all of sender's local snapshots (so receiver can cmp any sender event)
}

// ---------------------------------------------------------------------------
// Process
// ---------------------------------------------------------------------------

const BasePort = 9400

type Process struct {
	id     int
	allIDs []int       // sorted
	portOf map[int]int // id → port

	mu     sync.Mutex
	vector VClock // local vector clock

	// snapshot log: label → VClock, for causality comparison
	snapshots   map[string]VClock
	snapshotSeq int

	writerMu sync.Mutex
	writers  map[int]*bufio.Writer

	readyCh chan struct{}
}

func NewProcess(myID int, peerIDs []int) *Process {
	all := append([]int{myID}, peerIDs...)
	sort.Ints(all)

	portOf := make(map[int]int, len(all))
	for i, id := range all {
		portOf[id] = BasePort + i
	}

	vec := make(VClock, len(all))
	for _, id := range all {
		vec[id] = 0
	}

	return &Process{
		id:        myID,
		allIDs:    all,
		portOf:    portOf,
		vector:    vec,
		snapshots: make(map[string]VClock),
		writers:   make(map[int]*bufio.Writer),
		readyCh:   make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// Clock operations (caller must hold p.mu)
// ---------------------------------------------------------------------------

func (p *Process) tickLocked() {
	p.vector[p.id]++
}

// receiveLocked applies merge + self-increment.
func (p *Process) receiveLocked(incoming VClock) {
	p.vector.merge(incoming)
	p.vector[p.id]++
}

// snapshotLocked saves a labeled copy of the current vector.
func (p *Process) snapshotLocked(label string) {
	p.snapshots[label] = p.vector.copy()
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

func (p *Process) logf(format string, args ...interface{}) {
	p.mu.Lock()
	vec := p.vector.format(p.allIDs)
	p.mu.Unlock()
	fmt.Printf("[P%d | %s] "+format+"\n",
		append([]interface{}{p.id, vec}, args...)...)
}

func (p *Process) logfLocked(format string, args ...interface{}) {
	fmt.Printf("[P%d | %s] "+format+"\n",
		append([]interface{}{p.id, p.vector.format(p.allIDs)}, args...)...)
}

// ---------------------------------------------------------------------------
// Network
// ---------------------------------------------------------------------------

func (p *Process) sendTo(to int, msg Message) {
	data, _ := json.Marshal(msg)
	p.writerMu.Lock()
	w := p.writers[to]
	p.writerMu.Unlock()
	if w == nil {
		fmt.Printf("[P%d] ERROR: no connection to P%d\n", p.id, to)
		return
	}
	fmt.Fprintf(w, "%s\n", data)
	w.Flush()
}

func (p *Process) startListener() {
	port := p.portOf[p.id]
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("P%d: cannot listen on :%d: %v\n", p.id, port, err)
		os.Exit(1)
	}
	fmt.Printf("Process %d listening on :%d\n", p.id, port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handleConn(conn)
	}
}

func (p *Process) connectToPeers() {
	var wg sync.WaitGroup
	for _, id := range p.allIDs {
		if id == p.id {
			continue
		}
		wg.Add(1)
		go func(peerID int) {
			defer wg.Done()
			addr := fmt.Sprintf("localhost:%d", p.portOf[peerID])
			for {
				conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
				if err != nil {
					time.Sleep(400 * time.Millisecond)
					continue
				}
				w := bufio.NewWriter(conn)
				p.writerMu.Lock()
				p.writers[peerID] = w
				p.writerMu.Unlock()
				fmt.Printf("  Process %d: connected to Process %d (port %d)\n",
					p.id, peerID, p.portOf[peerID])
				return
			}
		}(id)
	}
	wg.Wait()
}

func (p *Process) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		<-p.readyCh
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		p.handleMessage(msg)
	}
}

// ---------------------------------------------------------------------------
// Event handlers
// ---------------------------------------------------------------------------

func (p *Process) InternalEvent() {
	p.mu.Lock()
	before := p.vector.format(p.allIDs)
	p.tickLocked()
	after := p.vector.format(p.allIDs)

	p.snapshotSeq++
	label := fmt.Sprintf("P%d-e%d", p.id, p.snapshotSeq)
	p.snapshotLocked(label)
	p.logfLocked("Internal event   %s → %s  [label: %s]", before, after, label)
	p.mu.Unlock()
}

func (p *Process) SendMessage(to int, text string) {
	found := false
	for _, id := range p.allIDs {
		if id == to {
			found = true
			break
		}
	}
	if !found || to == p.id {
		fmt.Printf("  Unknown peer ID: %d\n", to)
		return
	}

	p.mu.Lock()
	before := p.vector.format(p.allIDs)
	p.tickLocked() // V[self]++ before attaching
	msgVec := p.vector.copy()
	after := p.vector.format(p.allIDs)

	p.snapshotSeq++
	label := fmt.Sprintf("P%d-e%d", p.id, p.snapshotSeq)
	p.snapshotLocked(label)
	p.logfLocked("SEND → P%d  text=%q   %s → %s  [label: %s]",
		to, text, before, after, label)
	p.mu.Unlock()

	// Pack all local snapshots so the receiver can use cmp on any of our events
	p.mu.Lock()
	history := make(map[string]VClock, len(p.snapshots))
	for lbl, v := range p.snapshots {
		history[lbl] = v.copy()
	}
	p.mu.Unlock()

	p.sendTo(to, Message{From: p.id, Vector: msgVec, Text: text, SenderLabel: label, History: history})
}

func (p *Process) handleMessage(msg Message) {
	p.mu.Lock()
	before := p.vector.format(p.allIDs)

	// Store all of the sender's historical snapshots so this terminal
	// can run `cmp <any-sender-label> <local-label>` without needing
	// to be on the sender's terminal.
	for lbl, v := range msg.History {
		if _, exists := p.snapshots[lbl]; !exists { // don't overwrite local labels
			p.snapshots[lbl] = v.copy()
		}
	}

	p.receiveLocked(msg.Vector)
	after := p.vector.format(p.allIDs)

	p.snapshotSeq++
	label := fmt.Sprintf("P%d-e%d", p.id, p.snapshotSeq)
	p.snapshotLocked(label)

	p.logfLocked("RECV from P%d  text=%q   msg=%s  [sender: %s]",
		msg.From, msg.Text, msg.Vector.format(p.allIDs), msg.SenderLabel)
	p.logfLocked("  merge: max(%s, %s) then V[%d]++ → %s  [label: %s]",
		before, msg.Vector.format(p.allIDs), p.id, after, label)
	p.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Causality comparison
// ---------------------------------------------------------------------------

func (p *Process) compareSnapshots(labelA, labelB string) {
	p.mu.Lock()
	a, okA := p.snapshots[labelA]
	b, okB := p.snapshots[labelB]
	p.mu.Unlock()

	if !okA {
		fmt.Printf("  Unknown label: %s\n", labelA)
		return
	}
	if !okB {
		fmt.Printf("  Unknown label: %s\n", labelB)
		return
	}

	aStr := a.format(p.allIDs)
	bStr := b.format(p.allIDs)
	fmt.Printf("  %s = %s\n", labelA, aStr)
	fmt.Printf("  %s = %s\n", labelB, bStr)

	switch {
	case happensBefore(a, b):
		fmt.Printf("  Result: %s → %s  (%s happened-before %s)\n",
			labelA, labelB, labelA, labelB)
	case happensBefore(b, a):
		fmt.Printf("  Result: %s → %s  (%s happened-before %s)\n",
			labelB, labelA, labelB, labelA)
	case concurrent(a, b):
		fmt.Printf("  Result: %s || %s  (concurrent — no causal relationship)\n",
			labelA, labelB)
	default:
		fmt.Printf("  Result: identical vectors\n")
	}
}

func (p *Process) listSnapshots() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.snapshots) == 0 {
		fmt.Println("  No snapshots yet. Trigger events first.")
		return
	}

	// Separate local events from received peer snapshots
	local := []string{}
	peer := []string{}
	for seq := 1; seq <= p.snapshotSeq; seq++ {
		lbl := fmt.Sprintf("P%d-e%d", p.id, seq)
		if _, ok := p.snapshots[lbl]; ok {
			local = append(local, lbl)
		}
	}
	for lbl := range p.snapshots {
		if !strings.HasPrefix(lbl, fmt.Sprintf("P%d-", p.id)) {
			peer = append(peer, lbl)
		}
	}
	sort.Strings(peer)

	fmt.Println("  Local events:")
	for _, lbl := range local {
		fmt.Printf("    %-12s  %s\n", lbl, p.snapshots[lbl].format(p.allIDs))
	}
	if len(peer) > 0 {
		fmt.Println("  Received peer snapshots (usable in cmp):")
		for _, lbl := range peer {
			fmt.Printf("    %-12s  %s\n", lbl, p.snapshots[lbl].format(p.allIDs))
		}
	}
}

// ---------------------------------------------------------------------------
// Interactive loop
// ---------------------------------------------------------------------------

func (p *Process) printHelp() {
	fmt.Println("-------------------------------------------------------------")
	fmt.Println("Vector Clock Rules:")
	fmt.Println("  Internal event : V[self]++")
	fmt.Println("  Send           : V[self]++, attach full vector V")
	fmt.Println("  Receive        : V[k] = max(V[k], msg.V[k]) for all k,")
	fmt.Println("                   then V[self]++")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  i                    — internal event")
	fmt.Println("  send <id> [text]     — send message to process <id>")
	fmt.Println("  s    <id> [text]     — alias for send")
	fmt.Println("  ls                   — list local event snapshots")
	fmt.Println("  cmp  <lbl1> <lbl2>   — compare causality of two events")
	fmt.Println("                         e.g.: cmp P1-e1 P2-e2")
	fmt.Println("  q                    — quit")
	fmt.Println("-------------------------------------------------------------")
}

func (p *Process) runInteractive() {
	p.printHelp()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		p.mu.Lock()
		vec := p.vector.format(p.allIDs)
		p.mu.Unlock()
		fmt.Printf("\nP%d %s> ", p.id, vec)

		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "q", "quit":
			fmt.Printf("Process %d: exiting.\n", p.id)
			os.Exit(0)

		case "i", "internal":
			p.InternalEvent()

		case "s", "send":
			if len(parts) < 2 {
				fmt.Println("  Usage: send <peer-id> [text]")
				continue
			}
			to, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("  Invalid peer ID:", parts[1])
				continue
			}
			text := "hello"
			if len(parts) >= 3 {
				text = strings.Join(parts[2:], " ")
			}
			p.SendMessage(to, text)

		case "ls":
			p.listSnapshots()

		case "cmp":
			if len(parts) != 3 {
				fmt.Println("  Usage: cmp <label1> <label2>   e.g. cmp P1-e1 P2-e2")
				continue
			}
			p.compareSnapshots(parts[1], parts[2])

		default:
			fmt.Println("  Unknown command. Type 'q' to quit.")
		}
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <my-id> <peer-id> [peer-id ...]")
		fmt.Println()
		fmt.Println("Example (3 processes, IDs 1 2 3):")
		fmt.Println("  Terminal 1:  go run main.go 1 2 3")
		fmt.Println("  Terminal 2:  go run main.go 2 1 3")
		fmt.Println("  Terminal 3:  go run main.go 3 1 2")
		os.Exit(1)
	}

	myID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Invalid ID:", os.Args[1])
		os.Exit(1)
	}

	seen := map[int]bool{myID: true}
	peerIDs := []int{}
	for _, arg := range os.Args[2:] {
		id, err := strconv.Atoi(arg)
		if err != nil || seen[id] {
			fmt.Println("Invalid or duplicate peer ID:", arg)
			os.Exit(1)
		}
		seen[id] = true
		peerIDs = append(peerIDs, id)
	}

	p := NewProcess(myID, peerIDs)

	go p.startListener()
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("Process %d: connecting to %d peer(s)...\n", myID, len(peerIDs))
	p.connectToPeers()
	close(p.readyCh)
	fmt.Printf("Process %d: all peers connected. Vector clock starts at %s\n\n",
		myID, p.vector.format(p.allIDs))

	p.runInteractive()
}
