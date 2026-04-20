package main

import (
	"fmt"
	"sort"
	"sync"
)

// Định nghĩa các loại tin nhắn
type MsgType int

const (
	ENTER MsgType = iota
	ACK
	RELEASE
)

// Message tương đương với Tuple (clock, procID, TYPE) trong sách
type Message struct {
	Clock  int
	ProcID int
	Type   MsgType
}

// Giả lập hệ thống mạng (Network) để các Process gửi tin cho nhau
type Network interface {
	SendTo(targetIDs []int, msg Message)
	RecvFrom(procID int) Message // Giả lập nhận (Blocking)
}

// Process class tương đương trong Figure 5.10
type Process struct {
	mu         sync.Mutex // Cần Mutex vì Go xử lý đa luồng (Goroutines)
	procID     int
	otherProcs []int
	queue      []Message // The request queue
	clock      int       // The current logical clock
	net        Network   // Kênh giao tiếp
}

// Khởi tạo Process
func NewProcess(procID int, allProcIDs []int, net Network) *Process {
	otherProcs := []int{}
	for _, id := range allProcIDs {
		if id != procID {
			otherProcs = append(otherProcs, id)
		}
	}
	return &Process{
		procID:     procID,
		otherProcs: otherProcs,
		queue:      make([]Message, 0),
		clock:      0,
		net:        net,
	}
}

// ========================================================
// HÀM HỖ TRỢ: Sort the Queue (Logical Clock Ties Breaking)
// ========================================================
func (p *Process) cleanupQ() {
	// Sắp xếp queue dựa trên Clock, nếu Clock bằng nhau thì xét ProcID
	sort.Slice(p.queue, func(i, j int) bool {
		if p.queue[i].Clock == p.queue[j].Clock {
			return p.queue[i].ProcID < p.queue[j].ProcID // Tie breaker!
		}
		return p.queue[i].Clock < p.queue[j].Clock
	})
}

// ========================================================
// CODE THEO FIGURE 5.10 (a)
// ========================================================

func (p *Process) RequestToEnter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.clock++ // Increment clock value
	
	// Append request to queue
	reqMsg := Message{Clock: p.clock, ProcID: p.procID, Type: ENTER}
	p.queue = append(p.queue, reqMsg)
	p.cleanupQ() // Sort the queue

	// Send request to others
	p.net.SendTo(p.otherProcs, reqMsg)
}

func (p *Process) AckToEnter(requesterID int) {
	// Gọi trong lock của Receive nên không cần lock lại ở đây
	p.clock++ // Increment clock value
	ackMsg := Message{Clock: p.clock, ProcID: p.procID, Type: ACK}
	
	// Permit other (Gửi riêng cho người xin phép)
	p.net.SendTo([]int{requesterID}, ackMsg)
}

func (p *Process) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove all ACKs (and copy to new queue). Chỉ giữ lại ENTER
	var newQueue []Message
	for _, r := range p.queue {
		if r.Type == ENTER {
			newQueue = append(newQueue, r)
		}
	}
	p.queue = newQueue // Trang 262: "Cleaning up queue thus also involves removing old ACK/ALLOW messages"

	p.clock++ // Increment clock value
	relMsg := Message{Clock: p.clock, ProcID: p.procID, Type: RELEASE}
	
	p.net.SendTo(p.otherProcs, relMsg) // Multicast Release
}

func (p *Process) AllowedToEnter() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return false
	}

	// 1. Kiểm tra Lệnh ENTER của mình có ở trên TOP (đầu hàng) không?
	if p.queue[0].ProcID != p.procID || p.queue[0].Type != ENTER {
		return false
	}

	// 2. See who has sent a message (Kiểm tra đã nhận được phản hồi từ TẤT CẢ mọi người chưa)
	commProcs := make(map[int]bool)
	for i := 1; i < len(p.queue); i++ {
		commProcs[p.queue[i].ProcID] = true
	}

	return len(commProcs) == len(p.otherProcs)
}

// ========================================================
// CODE THEO FIGURE 5.10 (b) - Xử lý tin nhắn đến
// ========================================================

func (p *Process) Receive() {
	// Giả lập vòng lặp lắng nghe mạng (Chạy trong Goroutine riêng)
	for {
		msg := p.net.RecvFrom(p.procID) // Pick up any message (Blocking)
		
		p.mu.Lock()
		
		// Adjust clock value and increment
		if msg.Clock > p.clock {
			p.clock = msg.Clock
		}
		p.clock++

		switch msg.Type {
		case ENTER:
			p.queue = append(p.queue, msg) // Append an ENTER request
			p.AckToEnter(msg.ProcID)       // Unconditionally allow
		
		case ACK:
			p.queue = append(p.queue, msg) // Append a received ACK
		
		case RELEASE:
			// Just remove the first message (Lệnh ENTER cũ của người vừa Release)
			if len(p.queue) > 0 {
				p.queue = p.queue[1:]
			}
		}

		p.cleanupQ() // And sort and cleanup
		
		p.mu.Unlock()
	}
}

func main() {
	fmt.Println("Khởi tạo thuật toán Lamport Mutual Exclusion...")
	// Cần cài đặt interface Network và khởi chạy Goroutines ở đây để hệ thống chạy thực tế.
}

Nếu bạn có bất kỳ thắc mắc nào về luồng logic của đoạn code Go này so với sách, hay cách hàng đợi ở San Francisco và New York tự động chốt đơn (trong Canvas), hãy nói cho mình biết nhé!