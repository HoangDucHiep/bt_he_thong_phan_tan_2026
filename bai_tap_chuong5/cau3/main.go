// Ring Election Algorithm — Interactive TCP version
//
// Nodes form a logical ring in ascending ID order:  1 → 2 → 3 → ... → N → 1
// Each node only needs one outbound connection: to its successor.
// Inbound connection: from its predecessor.
//
// Usage (each in its own terminal):
//   go run main.go <my-id> <peer-id> [peer-id ...]
//
// Example (3 nodes with IDs 1, 2, 3):
//   Terminal 1:  go run main.go 1 2 3
//   Terminal 2:  go run main.go 2 1 3
//   Terminal 3:  go run main.go 3 1 2
//
// Commands after connecting:
//   e  — start ring election
//   s  — show status
//   q  — quit (peers auto-detect via missed heartbeat)
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
	ELECTION    MsgType = "ELECTION"    // ring token carrying max ID seen so far
	COORDINATOR MsgType = "COORDINATOR" // propagate new leader around ring
	HEARTBEAT   MsgType = "HEARTBEAT"   // leader → all nodes (broadcast)
)

type Message struct {
	Type      MsgType `json:"type"`
	From      int     `json:"from"`
	MaxID     int     `json:"max_id,omitempty"`    // ELECTION: highest ID seen
	Initiator int     `json:"initiator,omitempty"` // who started this election round
	LeaderID  int     `json:"leader_id,omitempty"` // COORDINATOR: the winner
}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

const BasePort = 9200

const (
	heartbeatInterval = 600 * time.Millisecond
	heartbeatTimeout  = 2 * time.Second
	watchdogInterval  = 500 * time.Millisecond
)

type Node struct {
	id     int
	allIDs []int       // sorted ascending — defines the ring order
	portOf map[int]int // nodeID → TCP port

	mu         sync.Mutex
	leaderID   int       // -1 = unknown
	inElection bool      // true only for the node that initiated the current round
	amLeader   bool
	lastHBtime time.Time

	writerMu sync.Mutex
	writers  map[int]*bufio.Writer // connections to ALL peers (for heartbeat broadcast)

	readyCh chan struct{} // closed when all outbound connections ready
}

func NewNode(myID int, peerIDs []int) *Node {
	all := append([]int{myID}, peerIDs...)
	sort.Ints(all)

	portOf := make(map[int]int, len(all))
	for i, id := range all {
		portOf[id] = BasePort + i
	}

	return &Node{
		id:       myID,
		allIDs:   all,
		portOf:   portOf,
		leaderID: -1,
		writers:  make(map[int]*bufio.Writer),
		readyCh:  make(chan struct{}),
	}
}

// successor returns the next node ID in the ring (wraps around).
func (nd *Node) successor() int {
	for i, id := range nd.allIDs {
		if id == nd.id {
			return nd.allIDs[(i+1)%len(nd.allIDs)]
		}
	}
	panic("own ID not in ring")
}

// ---------------------------------------------------------------------------
// Logging
// ---------------------------------------------------------------------------

func (nd *Node) logf(format string, args ...interface{}) {
	fmt.Printf("[Node %d] "+format+"\n", append([]interface{}{nd.id}, args...)...)
}

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
	w.Flush()
}

func (nd *Node) sendToSuccessor(msg Message) {
	succ := nd.successor()
	nd.logf("  --> forwarding %s to successor Node %d", msg.Type, succ)
	nd.sendTo(succ, msg)
}

// ---------------------------------------------------------------------------
// Network: listener + connect
// ---------------------------------------------------------------------------

func (nd *Node) startListener() {
	port := nd.portOf[nd.id]
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("Node %d: cannot listen on :%d: %v\n", nd.id, port, err)
		os.Exit(1)
	}
	fmt.Printf("Node %d listening on :%d\n", nd.id, port)
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
	for _, id := range nd.allIDs {
		if id == nd.id {
			continue
		}
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
		}(id)
	}
	wg.Wait()
}

func (nd *Node) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		<-nd.readyCh // block until all outbound connections are ready
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		nd.handleMessage(msg)
	}
}

// ---------------------------------------------------------------------------
// Ring Election: message handler
// ---------------------------------------------------------------------------

func (nd *Node) handleMessage(msg Message) {
	switch msg.Type {

	// ---- ELECTION token traveling around the ring -------------------------
	case ELECTION:
		// Update max ID seen so far
		newMax := msg.MaxID
		if nd.id > newMax {
			newMax = nd.id
		}
		nd.logf("Received ELECTION from Node %d  [maxID so far: %d → %d, initiator: Node %d]",
			msg.From, msg.MaxID, newMax, msg.Initiator)

		if nd.id == msg.Initiator {
			// The token completed a full trip around the ring.
			// newMax is the winner.
			nd.mu.Lock()
			nd.leaderID = newMax
			nd.inElection = false
			nd.amLeader = (newMax == nd.id)
			nd.mu.Unlock()

			nd.logf("Election complete — winner is Node %d", newMax)
			if newMax == nd.id {
				nd.logf("*** I am the new LEADER ***")
				go nd.heartbeatLoop()
			}

			// Propagate COORDINATOR around the ring
			nd.logf("Broadcasting COORDINATOR (leaderID=%d) around ring", newMax)
			nd.sendToSuccessor(Message{
				Type:      COORDINATOR,
				From:      nd.id,
				LeaderID:  newMax,
				Initiator: nd.id,
			})
		} else {
			// Forward with updated maxID
			nd.sendToSuccessor(Message{
				Type:      ELECTION,
				From:      nd.id,
				MaxID:     newMax,
				Initiator: msg.Initiator,
			})
		}

	// ---- COORDINATOR traveling around the ring ----------------------------
	case COORDINATOR:
		nd.mu.Lock()
		nd.leaderID = msg.LeaderID
		nd.inElection = false
		nd.amLeader = (msg.LeaderID == nd.id)
		nd.lastHBtime = time.Now()
		nd.mu.Unlock()

		nd.logf("*** New LEADER: Node %d ***", msg.LeaderID)
		if msg.LeaderID == nd.id {
			nd.logf("    (that's me!)")
			go nd.heartbeatLoop()
		}

		// Keep forwarding until the COORDINATOR comes back to the initiator
		if nd.id != msg.Initiator {
			nd.sendToSuccessor(Message{
				Type:      COORDINATOR,
				From:      nd.id,
				LeaderID:  msg.LeaderID,
				Initiator: msg.Initiator,
			})
		} else {
			nd.logf("COORDINATOR message completed full ring — done")
		}

	// ---- HEARTBEAT (leader → all, broadcast not ring) --------------------
	case HEARTBEAT:
		nd.mu.Lock()
		nd.lastHBtime = time.Now()
		nd.mu.Unlock()
	}
}

// ---------------------------------------------------------------------------
// Ring Election: initiate
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

	succ := nd.successor()
	nd.logf("--- Starting RING ELECTION ---")
	nd.logf("Ring order: %v", nd.allIDs)
	nd.logf("My successor: Node %d", succ)
	nd.logf("Sending ELECTION token to Node %d (maxID=%d)", succ, nd.id)

	nd.sendToSuccessor(Message{
		Type:      ELECTION,
		From:      nd.id,
		MaxID:     nd.id,
		Initiator: nd.id,
	})
}

// ---------------------------------------------------------------------------
// Heartbeat (leader only)
// ---------------------------------------------------------------------------

func (nd *Node) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		nd.mu.Lock()
		still := nd.amLeader
		nd.mu.Unlock()
		if !still {
			return
		}
		// Broadcast heartbeat to all peers directly
		for _, id := range nd.allIDs {
			if id != nd.id {
				nd.sendTo(id, Message{Type: HEARTBEAT, From: nd.id})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Watchdog (non-leader auto-detects leader failure)
// ---------------------------------------------------------------------------

func (nd *Node) watchdogLoop() {
	time.Sleep(3 * time.Second) // wait for initial election to settle
	for {
		time.Sleep(watchdogInterval)

		nd.mu.Lock()
		leaderID := nd.leaderID
		amLeader := nd.amLeader
		inElec := nd.inElection
		lastHB := nd.lastHBtime
		nd.mu.Unlock()

		if leaderID == -1 || amLeader || inElec || lastHB.IsZero() {
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

	role := "follower"
	if amLeader {
		role = "LEADER"
	}
	leader := fmt.Sprintf("Node %d", leaderID)
	if leaderID == -1 {
		leader = "unknown"
	}
	hb := ""
	if !amLeader && leaderID != -1 && !lastHB.IsZero() {
		hb = fmt.Sprintf(", last heartbeat %.1fs ago", time.Since(lastHB).Seconds())
	}
	fmt.Printf("  id=%d  role=%s  leader=%s  inElection=%v  successor=Node%d%s\n",
		nd.id, role, leader, inElec, nd.successor(), hb)
}

func (nd *Node) runInteractive() {
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Ring: %v (each node → next in list → wraps)\n", nd.allIDs)
	fmt.Printf("My successor: Node %d\n", nd.successor())
	fmt.Println("Commands:")
	fmt.Println("  e  — start ring election")
	fmt.Println("  s  — show status")
	fmt.Println("  q  — quit (peers will auto-detect and re-elect)")
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
		fmt.Println("Example (3 nodes, IDs 1 2 3):")
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

	nd := NewNode(myID, peerIDs)

	go nd.startListener()
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("Node %d: connecting to %d peer(s)...\n", myID, len(peerIDs))
	nd.connectToPeers()
	close(nd.readyCh)
	fmt.Printf("Node %d: all peers connected.\n\n", myID)

	go nd.watchdogLoop()
	nd.runInteractive()
}
