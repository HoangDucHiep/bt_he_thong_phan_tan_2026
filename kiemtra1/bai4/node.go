package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	heartbeatInterval  = 1 * time.Second
	leaderTimeout      = 3 * time.Second
	electionWaitTime   = leaderTimeout // chờ xem có Leader nào không trước khi ứng cử
	basePort           = 9100
)

type PeerInfo struct {
	id   int
	port int
}

type Node struct {
	id            int
	port          int
	peers         []PeerInfo
	currentLeader int
	leaderMu      sync.RWMutex
	lastHeartbeat map[int]time.Time
	hbMu          sync.Mutex
	stop          chan struct{}
}

func newNode(id, port int, peers []PeerInfo) *Node {
	return &Node{
		id:            id,
		port:          port,
		peers:         peers,
		currentLeader: 0,
		lastHeartbeat: make(map[int]time.Time),
		stop:          make(chan struct{}),
	}
}

func (n *Node) getLeader() int {
	n.leaderMu.RLock()
	defer n.leaderMu.RUnlock()
	return n.currentLeader
}

func (n *Node) setLeader(id int) {
	n.leaderMu.Lock()
	defer n.leaderMu.Unlock()
	n.currentLeader = id
}

func (n *Node) isLeader() bool {
	return n.getLeader() == n.id
}

func (n *Node) listen() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", n.port))
	if err != nil {
		fmt.Printf("[Node %d] Loi resolve addr: %v\n", n.id, err)
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("[Node %d] Loi listen UDP port %d: %v\n", n.id, n.port, err)
		os.Exit(1)
	}
	defer conn.Close()

	buf := make([]byte, 128)
	for {
		select {
		case <-n.stop:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			nr, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			n.handleMessage(string(buf[:nr]))
		}
	}
}

func (n *Node) handleMessage(msg string) {
	parts := strings.Split(strings.TrimSpace(msg), ":")
	if len(parts) < 2 {
		return
	}

	switch parts[0] {
	case "HB":
		senderID, _ := strconv.Atoi(parts[1])
		if senderID == 0 {
			return
		}
		n.hbMu.Lock()
		n.lastHeartbeat[senderID] = time.Now()
		n.hbMu.Unlock()

		// Nếu sender tự xưng là Leader và mình chưa có Leader → chấp nhận
		if len(parts) >= 3 && parts[2] == "L" {
			if n.getLeader() == 0 || n.getLeader() != senderID {
				n.setLeader(senderID)
				fmt.Printf("[Node %d] Nhan biet Leader: Node %d\n", n.id, senderID)
			}
		}

	case "VOTE":
		if len(parts) < 3 {
			return
		}
		newLeader, _ := strconv.Atoi(parts[2])
		if newLeader > 0 && newLeader != n.getLeader() {
			n.setLeader(newLeader)
			if newLeader == n.id {
				fmt.Printf("[Node %d] Duoc bau chon lam Leader moi!\n", n.id)
			} else {
				fmt.Printf("[Node %d] Chap nhan ket qua bau chon: Node %d la Leader moi.\n", n.id, newLeader)
			}
		}
	}
}

func (n *Node) sendHeartbeats() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			role := "F"
			if n.isLeader() {
				role = "L"
			}
			msg := fmt.Sprintf("HB:%d:%s", n.id, role)
			n.broadcast(msg)
		}
	}
}

func (n *Node) broadcast(msg string) {
	for _, peer := range n.peers {
		addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("localhost:%d", peer.port))
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			continue
		}
		conn.Write([]byte(msg))
		conn.Close()
	}
}

func (n *Node) initialElection() {
	fmt.Printf("[Node %d] Khoi dong, cho %v de kiem tra Leader hien tai...\n",
		n.id, electionWaitTime)

	time.Sleep(electionWaitTime)

	n.hbMu.Lock()
	hasLeader := false
	for _, peer := range n.peers {
		last, ok := n.lastHeartbeat[peer.id]
		if ok && time.Since(last) <= leaderTimeout {
			_ = last
			hasLeader = n.getLeader() != 0
			break
		}
	}
	n.hbMu.Unlock()

	if !hasLeader {
		fmt.Printf("[Node %d] Khong co Leader nao. Toi tu ung cu lam Leader!\n", n.id)
		n.setLeader(n.id)
		n.broadcast(fmt.Sprintf("VOTE:%d:%d", n.id, n.id))
	} else {
		fmt.Printf("[Node %d] Da co Leader: Node %d. Toi la Follower.\n", n.id, n.getLeader())
	}
}


func (n *Node) watchLeader() {
	time.Sleep(electionWaitTime + 500*time.Millisecond)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	leaderLost := false

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			leader := n.getLeader()

			if n.id == leader {
				continue 
			}

			if leader == 0 {
				continue
			}

			n.hbMu.Lock()
			last, ok := n.lastHeartbeat[leader]
			n.hbMu.Unlock()

			leaderAlive := ok && time.Since(last) <= leaderTimeout

			if !leaderAlive && !leaderLost {
				leaderLost = true
				fmt.Printf("[Node %d] Leader mat ket noi, bat dau quy trinh bau chon moi!\n", n.id)
				n.electNewLeader()
			} else if leaderAlive && leaderLost {
				leaderLost = false
				fmt.Printf("[Node %d] Leader (Node %d) da ket noi lai.\n", n.id, n.getLeader())
			}
		}
	}
}

func (n *Node) electNewLeader() {
	best := n.id

	n.hbMu.Lock()
	for _, peer := range n.peers {
		last, ok := n.lastHeartbeat[peer.id]
		if ok && time.Since(last) <= leaderTimeout {
			if peer.id > best {
				best = peer.id
			}
		}
	}
	n.hbMu.Unlock()

	n.setLeader(best)

	if best == n.id {
		fmt.Printf("[Node %d] Toi tro thanh Leader moi!\n", n.id)
	} else {
		fmt.Printf("[Node %d] Bau chon xong: Node %d la Leader moi.\n", n.id, best)
	}

	n.broadcast(fmt.Sprintf("VOTE:%d:%d", n.id, best))
}

func (n *Node) printStatus() {
	time.Sleep(electionWaitTime + 1*time.Second)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			role := "Follower"
			if n.isLeader() {
				role = "Leader"
			}
			fmt.Printf("[Node %d] Trang thai: %s | Leader hien tai: Node %d\n",
				n.id, role, n.getLeader())
		}
	}
}

func parsePeers(s string) []PeerInfo {
	var peers []PeerInfo
	if s == "" {
		return peers
	}
	for _, part := range strings.Split(s, ",") {
		kv := strings.Split(strings.TrimSpace(part), ":")
		if len(kv) != 2 {
			continue
		}
		id, _ := strconv.Atoi(kv[0])
		port, _ := strconv.Atoi(kv[1])
		peers = append(peers, PeerInfo{id: id, port: port})
	}
	return peers
}

func main() {
	idFlag := flag.Int("id", 0, "ID cua node nay (bat buoc)")
	portFlag := flag.Int("port", 0, "Port UDP (mac dinh: 9100+id)")
	peersFlag := flag.String("peers", "", "Danh sach peers: id:port,id:port,... (vd: 2:9102,3:9103)")
	flag.Parse()

	if *idFlag == 0 {
		fmt.Println("Cach dung:")
		fmt.Println("  ./node -id=1 -peers=2:9102,3:9103")
		fmt.Println("  ./node -id=2 -peers=1:9101,3:9103")
		fmt.Println("  ./node -id=3 -peers=1:9101,2:9102")
		os.Exit(1)
	}

	port := *portFlag
	if port == 0 {
		port = basePort + *idFlag
	}

	peers := parsePeers(*peersFlag)
	n := newNode(*idFlag, port, peers)

	fmt.Printf("[Node %d] Khoi dong tren port %d | Peers: %s\n", n.id, n.port, *peersFlag)

	go n.listen()
	go n.sendHeartbeats()
	go n.initialElection()
	go n.watchLeader()
	go n.printStatus()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Printf("\n[Node %d] Nhan tin hieu dung, tat node...\n", n.id)
	close(n.stop)
	time.Sleep(200 * time.Millisecond)
}
