package main

import (
	"context"
	"fmt"
	"time"

	pb "p2p_grpc/proto"
)

/* -- gRPC server --- */
// Join: New peer call to existing peer to join the network, return known peers list
func (p *PeerNode) Join(_ context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	// Lock to update peers map
	p.mu.Lock()
	defer p.mu.Unlock()

	// Collect known peers to return (include self)
	var knownPeers []*pb.PeerInfo
	knownPeers = append(knownPeers, &pb.PeerInfo{PeerId: p.id, PeerAddr: p.addr}) // add self info
	for id, addr := range p.peers {
		knownPeers = append(knownPeers, &pb.PeerInfo{PeerId: id, PeerAddr: addr})
	}

	// Add new peer to peers map
	p.peers[req.PeerId] = req.PeerAddr
	fmt.Printf("Peer %s joined via %s. Current peers: %v\n", req.PeerId, p.id, p.peers)

	return &pb.JoinResponse{
		Success:    true,
		Message:    fmt.Sprintf("Welcome %s! You have joined the network via %s.", req.PeerId, p.id),
		KnownPeers: knownPeers,
	}, nil
}

// NotifyJoin: Notify existing peers about new peer joining
func (p *PeerNode) NotifyJoin(_ context.Context, info *pb.PeerInfo) (*pb.AckResponse, error) {
	p.mu.Lock()
	p.peers[info.PeerId] = info.PeerAddr
	p.mu.Unlock()

	fmt.Printf("Peer %s notified about new peer %s. Current peers: %v\n", p.id, info.PeerId, p.peers)
	return &pb.AckResponse{
		Success: true,
	}, nil
}

// Leave: Peer call to existing peer to leave the network, remove from peers list
func (p *PeerNode) Leave(_ context.Context, req *pb.LeaveRequest) (*pb.AckResponse, error) {
	p.mu.Lock()
	delete(p.peers, req.PeerId)
	p.mu.Unlock()

	fmt.Printf("Peer %s left the network. Current peers: %v\n", req.PeerId, p.peers)
	return &pb.AckResponse{Success: true}, nil
}

// SendMessage: Nhận tin nhắn từ peer khác
func (p *PeerNode) SendMessage(_ context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	ts := time.Unix(req.Timestamp, 0).Format("15:04:05")
	fmt.Printf("\n[MSG %s] %s: %s\n> ", ts, req.SenderId, req.Content)
	return &pb.MessageResponse{Success: true, Message: "Delivered"}, nil
}
