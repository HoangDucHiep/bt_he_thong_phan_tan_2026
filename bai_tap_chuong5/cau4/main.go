// Lamport Logical Clock — Interactive TCP version
//
// Each process maintains a logical clock. Events follow Lamport's rules:
//   Internal event  : clock++
//   Send message    : clock++, attach timestamp to message
//   Receive message : clock = max(local, msg.timestamp) + 1
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
//   i                    — internal event (clock++)
//   send <id> <text>     — send a message to process <id>
//   s    <id> <text>     — alias for send
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
// Message
// ---------------------------------------------------------------------------

type Message struct {
	From      int    `json:"from"`
	Timestamp int    `json:"timestamp"`
	Text      string `json:"text"`
}

// ---------------------------------------------------------------------------
// Process
// ---------------------------------------------------------------------------

const BasePort = 9300

type Process struct {
	id     int
	allIDs []int
	portOf map[int]int

	mu    sync.Mutex
	clock int // Lamport logical clock

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

	return &Process{
		id:      myID,
		allIDs:  all,
		portOf:  portOf,
		clock:   0,
		writers: make(map[int]*bufio.Writer),
		readyCh: make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// Lamport clock operations (all require p.mu to be held)
// ---------------------------------------------------------------------------

// tickLocked increments the clock (internal event rule).
func (p *Process) tickLocked() {
	p.clock++
}

// updateLocked applies the receive rule: clock = max(local, received) + 1
func (p *Process) updateLocked(received int) {
	if received > p.clock {
		p.clock = received
	}
	p.clock++
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

func (p *Process) logf(format string, args ...interface{}) {
	p.mu.Lock()
	clk := p.clock
	p.mu.Unlock()
	fmt.Printf("[Process %d | Clock=%d] "+format+"\n",
		append([]interface{}{p.id, clk}, args...)...)
}

func (p *Process) logfLocked(format string, args ...interface{}) {
	fmt.Printf("[Process %d | Clock=%d] "+format+"\n",
		append([]interface{}{p.id, p.clock}, args...)...)
}

// ---------------------------------------------------------------------------
// Network: send
// ---------------------------------------------------------------------------

func (p *Process) sendTo(to int, msg Message) {
	data, _ := json.Marshal(msg)
	p.writerMu.Lock()
	w := p.writers[to]
	p.writerMu.Unlock()
	if w == nil {
		fmt.Printf("[Process %d] ERROR: no connection to Process %d\n", p.id, to)
		return
	}
	fmt.Fprintf(w, "%s\n", data)
	w.Flush()
}

// ---------------------------------------------------------------------------
// Network: listener + connect
// ---------------------------------------------------------------------------

func (p *Process) startListener() {
	port := p.portOf[p.id]
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("Process %d: cannot listen on :%d: %v\n", p.id, port, err)
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

// InternalEvent simulates a local computation step.
func (p *Process) InternalEvent() {
	p.mu.Lock()
	p.tickLocked()
	p.logfLocked("Internal event")
	p.mu.Unlock()
}

// SendMessage sends a message to the given peer.
func (p *Process) SendMessage(to int, text string) {
	// Check the peer is known
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
	p.tickLocked() // SEND increments clock before attaching timestamp
	ts := p.clock
	p.logfLocked("SEND to Process %d  text=%q  timestamp=%d", to, text, ts)
	p.mu.Unlock()

	p.sendTo(to, Message{From: p.id, Timestamp: ts, Text: text})
}

// handleMessage processes an incoming message (RECEIVE event).
func (p *Process) handleMessage(msg Message) {
	p.mu.Lock()
	before := p.clock
	p.updateLocked(msg.Timestamp) // max(local, msg) + 1
	p.logfLocked("RECV from Process %d  text=%q  msg_timestamp=%d  (clock %d → %d)",
		msg.From, msg.Text, msg.Timestamp, before, p.clock)
	p.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Interactive loop
// ---------------------------------------------------------------------------

func (p *Process) printHelp() {
	fmt.Println("----------------------------------------------------")
	fmt.Println("Lamport Clock Rules:")
	fmt.Println("  Internal event : clock++")
	fmt.Println("  Send           : clock++  then attach to message")
	fmt.Println("  Receive        : clock = max(local, msg.timestamp) + 1")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  i                    — trigger internal event")
	fmt.Println("  send <id> [text]     — send message to process <id>")
	fmt.Println("  s    <id> [text]     — alias for send")
	fmt.Println("  q                    — quit")
	fmt.Println("----------------------------------------------------")
}

func (p *Process) runInteractive() {
	p.printHelp()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		p.mu.Lock()
		clk := p.clock
		p.mu.Unlock()
		fmt.Printf("\nProcess %d (clock=%d)> ", p.id, clk)

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
	fmt.Printf("Process %d: all peers connected. Clock starts at 0.\n\n", myID)

	p.runInteractive()
}
