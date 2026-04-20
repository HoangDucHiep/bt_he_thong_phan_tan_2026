// Ricart-Agrawala Mutual Exclusion — Interactive TCP version
//
// Usage in separate terminals:
//   go run process.go <id> <n>
//
// Once all processes are connected, press [Enter] to request the critical
// section, or type 'q' to quit.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Message
// ---------------------------------------------------------------------------

type MsgType string

const (
	REQUEST MsgType = "REQUEST"
	REPLY   MsgType = "REPLY"
)

type Message struct {
	Type      MsgType `json:"type"`
	From      int     `json:"from"`
	Timestamp int     `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Process state
// ---------------------------------------------------------------------------

type State int

const (
	RELEASED State = iota
	WANTED
	HELD
)

func (s State) String() string {
	switch s {
	case RELEASED:
		return "RELEASED"
	case WANTED:
		return "WANTED"
	case HELD:
		return "HELD"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// Process
// ---------------------------------------------------------------------------

const BasePort = 9000

type Process struct {
	id       int
	n        int
	mu       sync.Mutex
	clock    int
	state    State
	reqTime  int
	replies  int
	deferred []bool

	replyCh chan struct{} // signalled when all N-1 replies received

	// outbound writers — populated after connectToPeers()
	writerMu sync.Mutex
	writers  map[int]*bufio.Writer

	// readyCh is closed once all outbound connections are established.
	// Incoming message handlers wait on this before processing messages,
	// ensuring we never try to reply before our writers map is ready.
	readyCh chan struct{}
}

func NewProcess(id, n int) *Process {
	return &Process{
		id:       id,
		n:        n,
		state:    RELEASED,
		deferred: make([]bool, n),
		replyCh:  make(chan struct{}, 1),
		writers:  make(map[int]*bufio.Writer),
		readyCh:  make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// Logging helpers
// ---------------------------------------------------------------------------

func (p *Process) logf(format string, args ...interface{}) {
	p.mu.Lock()
	clk := p.clock
	p.mu.Unlock()
	fmt.Printf("[Process %d | Clock %3d] "+format+"\n",
		append([]interface{}{p.id, clk}, args...)...)
}

// logfLocked may be called while p.mu is already held.
func (p *Process) logfLocked(format string, args ...interface{}) {
	fmt.Printf("[Process %d | Clock %3d] "+format+"\n",
		append([]interface{}{p.id, p.clock}, args...)...)
}

func (p *Process) tickLocked() { p.clock++ }

func (p *Process) updateClockLocked(ts int) {
	if ts > p.clock {
		p.clock = ts
	}
	p.clock++
}

// ---------------------------------------------------------------------------
// Network send
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
// Algorithm: handle incoming messages
// ---------------------------------------------------------------------------

func (p *Process) handleMessage(msg Message) {
	p.mu.Lock()
	p.updateClockLocked(msg.Timestamp)

	switch msg.Type {
	case REQUEST:
		p.logfLocked("Received REQUEST from Process %d (ts=%d)", msg.From, msg.Timestamp)

		weHavePriority := p.state == WANTED &&
			(p.reqTime < msg.Timestamp ||
				(p.reqTime == msg.Timestamp && p.id < msg.From))

		if p.state == HELD || weHavePriority {
			p.deferred[msg.From] = true
			p.logfLocked("Deferring REPLY to Process %d  [our state=%s, our ts=%d]",
				msg.From, p.state, p.reqTime)
			p.mu.Unlock()
		} else {
			p.tickLocked()
			reply := Message{Type: REPLY, From: p.id, Timestamp: p.clock}
			p.logfLocked("Sending REPLY to Process %d", msg.From)
			p.mu.Unlock()
			p.sendTo(msg.From, reply)
		}

	case REPLY:
		p.replies++
		p.logfLocked("Received REPLY from Process %d  (%d/%d)", msg.From, p.replies, p.n-1)
		if p.replies == p.n-1 {
			select {
			case p.replyCh <- struct{}{}:
			default:
			}
		}
		p.mu.Unlock()
	}
}

func (p *Process) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Wait until all outbound connections are ready before processing.
		// After readyCh is closed this returns immediately every time.
		<-p.readyCh

		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		p.handleMessage(msg)
	}
}

// ---------------------------------------------------------------------------
// Network setup
// ---------------------------------------------------------------------------

func (p *Process) startListener() {
	addr := fmt.Sprintf(":%d", BasePort+p.id)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Process %d: cannot listen on %s: %v\n", p.id, addr, err)
		os.Exit(1)
	}
	fmt.Printf("Process %d listening on %s\n", p.id, addr)
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
	for i := 0; i < p.n; i++ {
		if i == p.id {
			continue
		}
		wg.Add(1)
		go func(peer int) {
			defer wg.Done()
			addr := fmt.Sprintf("localhost:%d", BasePort+peer)
			for {
				conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
				if err != nil {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				w := bufio.NewWriter(conn)
				p.writerMu.Lock()
				p.writers[peer] = w
				p.writerMu.Unlock()
				fmt.Printf("  connected to Process %d (%s)\n", peer, addr)
				return
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Algorithm: CS entry / exit
// ---------------------------------------------------------------------------

func (p *Process) RequestCS() {
	p.mu.Lock()
	p.tickLocked()
	p.state = WANTED
	p.reqTime = p.clock
	p.replies = 0
	ts := p.clock
	p.logfLocked("Requesting CS — broadcasting REQUEST (ts=%d)", ts)
	p.mu.Unlock()

	for i := 0; i < p.n; i++ {
		if i != p.id {
			p.sendTo(i, Message{Type: REQUEST, From: p.id, Timestamp: ts})
		}
	}
}

func (p *Process) EnterCS() {
	<-p.replyCh
	p.mu.Lock()
	p.state = HELD
	p.logfLocked("==> ENTERING CRITICAL SECTION")
	p.mu.Unlock()
}

func (p *Process) ExitCS() {
	p.mu.Lock()
	p.state = RELEASED
	p.tickLocked()
	ts := p.clock
	p.logfLocked("<== EXITING CRITICAL SECTION")

	toReply := []int{}
	for i, d := range p.deferred {
		if d {
			toReply = append(toReply, i)
			p.deferred[i] = false
		}
	}
	p.mu.Unlock()

	for _, dest := range toReply {
		p.logf("Sending deferred REPLY to Process %d", dest)
		p.sendTo(dest, Message{Type: REPLY, From: p.id, Timestamp: ts})
	}
}

// ---------------------------------------------------------------------------
// Interactive loop
// ---------------------------------------------------------------------------

func (p *Process) runInteractive() {
	fmt.Println("---------------------------------------------------")
	fmt.Println("Commands:")
	fmt.Println("  [Enter]  — request critical section")
	fmt.Println("  q        — quit")
	fmt.Println("---------------------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\nProcess %d> ", p.id)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())

		if line == "q" || line == "quit" {
			fmt.Printf("Process %d: exiting.\n", p.id)
			os.Exit(0)
		}

		// Any other input (including empty Enter) → request CS
		p.mu.Lock()
		current := p.state
		p.mu.Unlock()

		if current != RELEASED {
			fmt.Printf("Process %d is currently %s — please wait.\n", p.id, current)
			continue
		}

		// Reset reply channel before each new round
		select {
		case <-p.replyCh:
		default:
		}

		go func() {
			p.RequestCS()
			p.EnterCS()

			fmt.Printf("\n    *** Process %d is inside the critical section ***\n", p.id)
			fmt.Printf("    (simulating work for 1 second...)\n")
			time.Sleep(1 * time.Second)

			p.ExitCS()
			fmt.Printf("\nProcess %d> ", p.id) // re-print prompt after async work
		}()
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run process.go <id> <n>")
		fmt.Println("  id  process ID (0-indexed)")
		fmt.Println("  n   total number of processes")
		os.Exit(1)
	}

	id, err1 := strconv.Atoi(os.Args[1])
	n, err2 := strconv.Atoi(os.Args[2])
	if err1 != nil || err2 != nil || id < 0 || id >= n || n < 2 {
		fmt.Println("Invalid arguments.")
		os.Exit(1)
	}

	p := NewProcess(id, n)

	// Start listener first so peers can connect to us
	go p.startListener()
	time.Sleep(50 * time.Millisecond)

	// Connect outbound to all peers
	fmt.Printf("Process %d: connecting to %d peer(s)...\n", id, n-1)
	p.connectToPeers()

	// Signal that we are fully connected — unblocks any buffered incoming messages
	close(p.readyCh)
	fmt.Printf("Process %d: all peers connected.\n\n", id)

	// Hand off to interactive loop
	p.runInteractive()
}
