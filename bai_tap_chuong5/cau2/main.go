// Bully Election Algorithm — Interactive TCP version with auto-detection
//
// Usage (each in its own terminal):
//   go run main.go <my-id> <peer-id> <peer-id> ...
//
// Example with 3 nodes (IDs: 1, 2, 3):
//   Terminal 1:  go run main.go 1 2 3
//   Terminal 2:  go run main.go 2 1 3
//   Terminal 3:  go run main.go 3 1 2
//
// The node with the highest ID wins elections.
// When the leader exits, other nodes detect the missing heartbeat
// and automatically start a new election.
//
// Commands (after all nodes connect):
//   e  — manually start election
//   s  — show current status
//   q  — quit (other nodes will detect this and re-elect)
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
// Messages
// ---------------------------------------------------------------------------

type MsgType string

const (
	ELECTION    MsgType = "ELECTION"    // "I'm starting an election"
	OK          MsgType = "OK"          // "I'm alive and higher — I'll take over"
	COORDINATOR MsgType = "COORDINATOR" // "I am the new leader"
	HEARTBEAT   MsgType = "HEARTBEAT"   // leader → all: "I'm still alive"
)

type Message struct {
	Type MsgType `json:"type"`
	From int     `json:"from"`
}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

const BasePort = 9100

// Heartbeat / watchdog timing
const (
	heartbeatInterval = 600 * time.Millisecond
	heartbeatTimeout  = 2 * time.Second
	watchdogInterval  = 500 * time.Millisecond
)

type Node struct {
	id     int
	allIDs []int          // sorted list of all node IDs (self included)
	portOf map[int]int    // nodeID → TCP port
	peers  []int          // IDs of other nodes

	mu          sync.Mutex
	leaderID    int       // -1 = unknown
	inElection  bool
	amLeader    bool
	lastHBtime  time.Time // last heartbeat received from leader

	writerMu sync.Mutex
	writers  map[int]*bufio.Writer

	// readyCh is closed once all outbound connections are ready.
	readyCh chan struct{}
	// okCh is signalled each time we receive an OK during an active election.
	okCh chan struct{}
}

func NewNode(myID int, peerIDs []int) *Node {
	all := append([]int{myID}, peerIDs...)
	sort.Ints(all)

	portOf := make(map[int]int, len(all))
	for i, id := range all {
		portOf[id] = BasePort + i
	}

	return &Node{
		id:      myID,
		allIDs:  all,
		portOf:  portOf,
		peers:   peerIDs,
		leaderID: -1,
		writers: make(map[int]*bufio.Writer),
		readyCh: make(chan struct{}),
		okCh:    make(chan struct{}, len(all)),
	}
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

func (nd *Node) logf(format string, args ...interface{}) {
	fmt.Printf("[Node %d] "+format+"\n", append([]interface{}{nd.id}, args...)...)
}

// logfLocked may be called while nd.mu is already held.
func (nd *Node) logfLocked(format string, args ...interface{}) {
	nd.logf(format, args...)
}

// ---------------------------------------------------------------------------
// Network: send
// ---------------------------------------------------------------------------

func (nd *Node) sendTo(to int, msg Message) {
	data, _ := json.Marshal(msg)
	nd.writerMu.Lock()
	w := nd.writers[to]
	nd.writerMu.Unlock()
	if w == nil {
		return
	}
	fmt.Fprintf(w, "%s\n", data)
	w.Flush() // ignore error — broken pipe means peer is down
}

// ---------------------------------------------------------------------------
// Network: listener + connect
// ---------------------------------------------------------------------------

func (nd *Node) startListener() {
	port := nd.portOf[nd.id]
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Node %d: cannot listen on %s: %v\n", nd.id, addr, err)
		os.Exit(1)
	}
	fmt.Printf("Node %d listening on %s\n", nd.id, addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go nd.handleConn(conn)
	}
}

func (nd *Node) connectToPeers() {
	var wg sync.WaitGroup
	for _, peer := range nd.peers {
		wg.Add(1)
		go func(peerID int) {
			defer wg.Done()
			addr := fmt.Sprintf("localhost:%d", nd.portOf[peerID])
			for {
				conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
				if err != nil {
					time.Sleep(400 * time.Millisecond)
					continue
				}
				w := bufio.NewWriter(conn)
				nd.writerMu.Lock()
				nd.writers[peerID] = w
				nd.writerMu.Unlock()
				fmt.Printf("  Node %d: connected to Node %d (port %d)\n", nd.id, peerID, nd.portOf[peerID])
				return
			}
		}(peer)
	}
	wg.Wait()
}

func (nd *Node) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		<-nd.readyCh // wait until outbound connections are ready
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		nd.handleMessage(msg)
	}
}

// ---------------------------------------------------------------------------
// Bully algorithm: message handler
// ---------------------------------------------------------------------------

func (nd *Node) handleMessage(msg Message) {
	switch msg.Type {

	case ELECTION:
		nd.logf("Received ELECTION from Node %d", msg.From)
		nd.logf("Sending OK to Node %d", msg.From)
		nd.sendTo(msg.From, Message{Type: OK, From: nd.id})
		go nd.startElection()

	case OK:
		nd.logf("Received OK from Node %d — a higher node is handling it", msg.From)
		select {
		case nd.okCh <- struct{}{}:
		default:
		}

	case COORDINATOR:
		nd.mu.Lock()
		old := nd.leaderID
		nd.leaderID = msg.From
		nd.inElection = false
		nd.amLeader = (msg.From == nd.id)
		nd.lastHBtime = time.Now() // reset watchdog
		nd.mu.Unlock()
		if old != msg.From {
			nd.logf("*** Node %d is the new LEADER ***", msg.From)
			if msg.From == nd.id {
				nd.logf("    (that's me!)")
			}
		}

	case HEARTBEAT:
		nd.mu.Lock()
		nd.lastHBtime = time.Now()
		nd.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// Bully algorithm: election logic
// ---------------------------------------------------------------------------

func (nd *Node) startElection() {
	nd.mu.Lock()
	if nd.inElection {
		nd.mu.Unlock()
		nd.logf("Election already in progress — skipping")
		return
	}
	nd.inElection = true
	nd.amLeader = false
	nd.mu.Unlock()

	nd.logf("--- Starting ELECTION ---")

	// Nodes with higher ID
	higher := []int{}
	for _, id := range nd.allIDs {
		if id > nd.id {
			higher = append(higher, id)
		}
	}

	if len(higher) == 0 {
		nd.becomeLeader()
		return
	}

	// Drain stale OKs
	for {
		select {
		case <-nd.okCh:
		default:
			goto drained
		}
	}
drained:

	for _, peer := range higher {
		nd.logf("Sending ELECTION to Node %d", peer)
		nd.sendTo(peer, Message{Type: ELECTION, From: nd.id})
	}

	select {
	case <-nd.okCh:
		nd.logf("Got OK — stepping back, waiting for COORDINATOR...")
		nd.mu.Lock()
		nd.inElection = false
		nd.mu.Unlock()

	case <-time.After(2 * time.Second):
		nd.becomeLeader()
	}
}

func (nd *Node) becomeLeader() {
	nd.mu.Lock()
	nd.leaderID = nd.id
	nd.inElection = false
	nd.amLeader = true
	nd.lastHBtime = time.Now()
	nd.logfLocked("No higher node responded — I become the LEADER")
	nd.mu.Unlock()

	for _, peer := range nd.peers {
		nd.sendTo(peer, Message{Type: COORDINATOR, From: nd.id})
	}
	nd.logf("*** I am the LEADER (Node %d) ***", nd.id)

	go nd.heartbeatLoop()
}

// ---------------------------------------------------------------------------
// Heartbeat sender (runs only on leader)
// ---------------------------------------------------------------------------

func (nd *Node) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		nd.mu.Lock()
		still := nd.amLeader
		nd.mu.Unlock()
		if !still {
			return // lost leadership, stop sending
		}
		for _, peer := range nd.peers {
			nd.sendTo(peer, Message{Type: HEARTBEAT, From: nd.id})
		}
	}
}

// ---------------------------------------------------------------------------
// Watchdog (runs on every node, triggers election if leader heartbeat stops)
// ---------------------------------------------------------------------------

func (nd *Node) watchdogLoop() {
	// Give the cluster time to elect a first leader before watching
	time.Sleep(3 * time.Second)

	for {
		time.Sleep(watchdogInterval)

		nd.mu.Lock()
		leaderID := nd.leaderID
		amLeader := nd.amLeader
		inElec := nd.inElection
		lastHB := nd.lastHBtime
		nd.mu.Unlock()

		// Only watch when there's a known external leader
		if leaderID == -1 || amLeader || inElec {
			continue
		}

		if lastHB.IsZero() {
			continue
		}

		elapsed := time.Since(lastHB)
		if elapsed > heartbeatTimeout {
			nd.logf("No heartbeat from leader Node %d for %.1fs — starting election",
				leaderID, elapsed.Seconds())
			nd.mu.Lock()
			nd.leaderID = -1
			nd.mu.Unlock()
			go nd.startElection()
		}
	}
}

// ---------------------------------------------------------------------------
// Interactive loop
// ---------------------------------------------------------------------------

func (nd *Node) printStatus() {
	nd.mu.Lock()
	leaderID := nd.leaderID
	amLeader := nd.amLeader
	inElec := nd.inElection
	lastHB := nd.lastHBtime
	nd.mu.Unlock()

	role := "non-leader"
	if amLeader {
		role = "LEADER"
	}

	leader := fmt.Sprintf("Node %d", leaderID)
	if leaderID == -1 {
		leader = "unknown"
	}

	hbInfo := ""
	if !amLeader && leaderID != -1 && !lastHB.IsZero() {
		hbInfo = fmt.Sprintf(", last heartbeat %.1fs ago", time.Since(lastHB).Seconds())
	}

	fmt.Printf("  ID=%d  role=%s  leader=%s  inElection=%v%s\n",
		nd.id, role, leader, inElec, hbInfo)
}

func (nd *Node) runInteractive() {
	fmt.Println("--------------------------------------------------")
	fmt.Println("Commands:")
	fmt.Println("  e  — manually start election")
	fmt.Println("  s  — show status")
	fmt.Println("  q  — quit (other nodes will auto-detect and re-elect)")
	fmt.Println("--------------------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\nNode %d> ", nd.id)
		if !scanner.Scan() {
			break
		}
		switch strings.TrimSpace(scanner.Text()) {
		case "q", "quit":
			fmt.Printf("Node %d: exiting.\n", nd.id)
			os.Exit(0)
		case "s", "status":
			nd.printStatus()
		case "e", "election", "":
			go nd.startElection()
		default:
			fmt.Println("  Unknown command. Use: e / s / q")
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
		fmt.Println("Example (3 nodes with IDs 1, 2, 3):")
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

	peerIDs := []int{}
	seen := map[int]bool{myID: true}
	for _, arg := range os.Args[2:] {
		id, err := strconv.Atoi(arg)
		if err != nil || seen[id] {
			fmt.Println("Invalid or duplicate peer ID:", arg)
			os.Exit(1)
		}
		seen[id] = true
		peerIDs = append(peerIDs, id)
	}

	nd := NewNode(myID, peerIDs)

	go nd.startListener()
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("Node %d: connecting to %d peer(s)...\n", myID, len(peerIDs))
	nd.connectToPeers()
	close(nd.readyCh)
	fmt.Printf("Node %d: all peers connected.\n", myID)

	// Port info (helpful for first run)
	fmt.Printf("Node %d: port map: ", myID)
	for _, id := range nd.allIDs {
		fmt.Printf("Node%d→%d ", id, nd.portOf[id])
	}
	fmt.Println()

	go nd.watchdogLoop()

	nd.runInteractive()
}
