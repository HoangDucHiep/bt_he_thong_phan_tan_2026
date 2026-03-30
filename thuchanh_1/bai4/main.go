package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	NumProcesses = 4
	BasePort     = 5000
	Timeout      = 5 * time.Second
)

type MessageType string

const (
	MsgRequest     MessageType = "REQUEST"
	MsgReply       MessageType = "REPLY"
	MsgElection    MessageType = "ELECTION"
	MsgOk          MessageType = "OK"
	MsgCoordinator MessageType = "COORDINATOR"
)

type Message struct {
	Type      MessageType `json:"type"`
	Timestamp int         `json:"timestamp,omitempty"`
	FromID    int         `json:"fromID"`
}

type Node struct {
	ID             int
	Port           int
	Peers          map[int]string // địa chỉ cố định
	logicalClock   int
	mu             sync.Mutex
	state          string // IDLE, REQUESTING, IN_CS
	deferred       []Message
	pendingReplies int
	csWait         chan bool
	logFile        string

	// Bully
	electionMu  sync.Mutex
	isElection  bool
	coordinator int
	alive       map[int]bool
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func NewNode(id int) *Node {
	n := &Node{
		ID:          id,
		Port:        BasePort + id,
		logFile:     "distributed_log.txt",
		state:       "IDLE",
		coordinator: -1,
		alive:       make(map[int]bool),
	}
	for i := 0; i < NumProcesses; i++ {
		n.alive[i] = true
	}
	n.Peers = make(map[int]string)
	for i := 0; i < NumProcesses; i++ {
		if i != id {
			n.Peers[i] = fmt.Sprintf("127.0.0.1:%d", BasePort+i)
		}
	}
	if id == NumProcesses-1 {
		n.coordinator = id
		fmt.Printf("[P%d] is the initial coordinator\n", id)
	}
	return n
}

func (n *Node) Start() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
	if err != nil {
		fmt.Printf("[P%d] Listen error: %v\n", n.ID, err)
		return
	}
	go n.listen(ln)
	go n.simulateWork()

	fmt.Printf("[P%d] started on port %d\n", n.ID, n.Port)
	select {}
}

func (n *Node) listen(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go n.handleConnection(conn)
	}
}

func (n *Node) handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil {
			n.handleMessage(msg)
		}
	}
}

func (n *Node) sendMessage(toID int, msg Message) error {
	if !n.alive[toID] {
		return fmt.Errorf("node %d is dead", toID)
	}
	addr := n.Peers[toID]
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		n.handleCrash(toID)
		return err
	}
	defer conn.Close()

	b, _ := json.Marshal(msg)
	_, err = conn.Write(append(b, '\n'))
	return err
}

func (n *Node) handleMessage(msg Message) {
	n.mu.Lock()
	n.logicalClock = max(n.logicalClock, msg.Timestamp) + 1
	n.mu.Unlock()

	switch msg.Type {
	case MsgRequest:
		n.handleRequest(msg)
	case MsgReply:
		n.handleReply()
	case MsgElection:
		n.handleElection(msg)
	case MsgOk:
		n.handleOk(msg)
	case MsgCoordinator:
		n.handleCoordinator(msg)
	}
}

// RICART-AGRAWALA
func (n *Node) RequestCS() {
	n.mu.Lock()
	n.logicalClock++
	ts := n.logicalClock
	n.state = "REQUESTING"
	n.csWait = make(chan bool, 1)

	// Chỉ đếm và gửi cho các node còn sống
	n.pendingReplies = 0
	for pid := range n.Peers {
		if n.alive[pid] {
			n.pendingReplies++
		}
	}
	n.mu.Unlock()

	if n.pendingReplies == 0 {
		n.enterCS()
		return
	}

	req := Message{Type: MsgRequest, Timestamp: ts, FromID: n.ID}
	for pid := range n.Peers {
		if n.alive[pid] {
			go n.sendMessage(pid, req)
		}
	}

	timer := time.NewTimer(Timeout)
	select {
	case <-n.csWait:
		n.enterCS()
	case <-timer.C:
		n.mu.Lock()
		n.state = "IDLE"
		n.mu.Unlock()
		n.handleCrashDetection()
	}
}

func (n *Node) handleReply() {
	n.mu.Lock()
	if n.state == "REQUESTING" {
		n.pendingReplies--
		if n.pendingReplies == 0 {
			n.state = "IN_CS"
			if n.csWait != nil {
				select {
				case n.csWait <- true:
				default:
				}
			}
		}
	}
	n.mu.Unlock()
}

func (n *Node) handleRequest(msg Message) {
	n.mu.Lock()
	defer n.mu.Unlock()

	shouldReply := false
	if n.state == "IDLE" {
		shouldReply = true
	} else if n.state == "REQUESTING" {
		if msg.Timestamp < n.logicalClock || (msg.Timestamp == n.logicalClock && msg.FromID < n.ID) {
			shouldReply = true
		} else {
			n.deferred = append(n.deferred, msg)
		}
	} else if n.state == "IN_CS" {
		n.deferred = append(n.deferred, msg)
	}

	if shouldReply {
		reply := Message{Type: MsgReply, FromID: n.ID}
		go n.sendMessage(msg.FromID, reply)
	}
}

func (n *Node) enterCS() {
	n.mu.Lock()
	fmt.Printf("[P%d] entered critical section (logical time %d)\n", n.ID, n.logicalClock)
	f, _ := os.OpenFile(n.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	f.WriteString(fmt.Sprintf("P%d entered CS at logical time %d\n", n.ID, n.logicalClock))
	f.Close()
	n.mu.Unlock()

	time.Sleep(2 * time.Second)
	n.releaseCS()
}

func (n *Node) releaseCS() {
	n.mu.Lock()
	n.state = "IDLE"
	for _, d := range n.deferred {
		reply := Message{Type: MsgReply, FromID: n.ID}
		go n.sendMessage(d.FromID, reply)
	}
	n.deferred = nil
	n.mu.Unlock()
	fmt.Printf("[P%d] exited critical section\n", n.ID)
}

// ==================== CRASH & BULLY (đã fix race condition) ====================
func (n *Node) handleCrash(toID int) {
	n.mu.Lock()
	if n.alive[toID] {
		n.alive[toID] = false
		fmt.Printf("[P%d] detected crash of P%d\n", n.ID, toID)
	}
	n.mu.Unlock()
	n.startElection()
}

func (n *Node) handleCrashDetection() {
	fmt.Printf("[P%d] timeout waiting for replies, starting election\n", n.ID)
	n.startElection()
}

func (n *Node) startElection() {
	n.electionMu.Lock()
	if n.isElection {
		n.electionMu.Unlock()
		fmt.Printf("[P%d] Election already in progress, skipping\n", n.ID)
		return
	}
	n.isElection = true
	n.electionMu.Unlock()

	fmt.Printf("[P%d] Starting Bully election\n", n.ID)

	// copy alive để tránh race
	n.mu.Lock()
	aliveCopy := make(map[int]bool)
	for k, v := range n.alive {
		aliveCopy[k] = v
	}
	n.mu.Unlock()

	sent := false
	for pid := range n.Peers {
		if pid > n.ID && aliveCopy[pid] {
			elect := Message{Type: MsgElection, FromID: n.ID}
			if n.sendMessage(pid, elect) == nil {
				sent = true
				fmt.Printf("[P%d] Sent ELECTION to higher P%d\n", n.ID, pid)
			}
		}
	}

	if !sent {
		fmt.Printf("[P%d] No higher alive process → becoming coordinator immediately\n", n.ID)
		n.becomeCoordinator()
		return
	}

	// timeout fallback
	go func() {
		time.Sleep(Timeout)
		n.electionMu.Lock()
		if n.isElection {
			fmt.Printf("[P%d] Election timeout → becoming coordinator\n", n.ID)
			n.becomeCoordinator()
		}
		n.electionMu.Unlock()
	}()
}

func (n *Node) handleElection(msg Message) {
	ok := Message{Type: MsgOk, FromID: n.ID}
	go n.sendMessage(msg.FromID, ok)

	if msg.FromID < n.ID {
		n.electionMu.Lock()
		wasElection := n.isElection
		n.isElection = true
		n.electionMu.Unlock()

		if !wasElection {
			n.startElection()
		}
	}
}

func (n *Node) handleOk(msg Message) {
	n.electionMu.Lock()
	n.isElection = false
	n.electionMu.Unlock()
	fmt.Printf("[P%d] Received OK from higher process → waiting for coordinator\n", n.ID)
}

func (n *Node) becomeCoordinator() {
	n.mu.Lock()
	n.coordinator = n.ID
	n.mu.Unlock()

	n.electionMu.Lock()
	n.isElection = false
	n.electionMu.Unlock()

	fmt.Printf("[P%d] I AM THE NEW COORDINATOR!\n", n.ID)

	coord := Message{Type: MsgCoordinator, FromID: n.ID}
	for pid := range n.Peers {
		if pid < n.ID && n.alive[pid] {
			go n.sendMessage(pid, coord)
		}
	}
}

func (n *Node) handleCoordinator(msg Message) {
	n.mu.Lock()
	n.coordinator = msg.FromID
	n.mu.Unlock()

	n.electionMu.Lock()
	n.isElection = false
	n.electionMu.Unlock()

	fmt.Printf("[P%d] New coordinator is P%d\n", n.ID, msg.FromID)
}

func (n *Node) simulateWork() {
	time.Sleep(3 * time.Second)
	for {
		time.Sleep(6*time.Second + time.Duration(n.ID*800)*time.Millisecond)
		n.mu.Lock()
		if n.state == "IDLE" {
			n.mu.Unlock()
			fmt.Printf("[P%d] wants to enter critical section...\n", n.ID)
			n.RequestCS()
		} else {
			n.mu.Unlock()
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <id>  (id = 0,1,2,3)")
		return
	}
	id, _ := strconv.Atoi(os.Args[1])
	if id < 0 || id >= NumProcesses {
		fmt.Println("ID phải từ 0 đến 3")
		return
	}

	node := NewNode(id)
	node.Start()
}
