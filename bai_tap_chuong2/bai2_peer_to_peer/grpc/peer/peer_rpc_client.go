package main

import (
	"context"
	"fmt"
	pb "p2p_grpc/proto"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

/* gRPC client */
func dialPeer(addr string) (pb.PeerServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, nil, err
	}

	return pb.NewPeerServiceClient(conn), conn, nil
}

// Join / Leave network
func (p *PeerNode) joinNetwork(bootstrapAddr string) error {
	client, conn, err := dialPeer(bootstrapAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to bootstrap peer: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// call to join network, get known peers list
	resp, err := client.Join(ctx, &pb.JoinRequest{
		PeerId:   p.id,
		PeerAddr: p.addr,
	})

	if err != nil {
		return fmt.Errorf("join failed: %v", err)
	}

	// Save all known peers to local peers map
	p.mu.Lock()
	for _, peer := range resp.KnownPeers {
		if peer.PeerId != p.id { // skip self
			p.peers[peer.PeerId] = peer.PeerAddr
		}
	}
	p.mu.Unlock()

	fmt.Printf("[*] %s\n", resp.Message)
	fmt.Printf("[*] Found %d peer(s) in network\n", len(p.peers))

	// Notify all known peers about new peer joining (except bootstrap peer)
	p.mu.RLock()
	peersToNotify := make(map[string]string)
	for id, addr := range p.peers {
		if addr != bootstrapAddr {
			peersToNotify[id] = addr
		}
	}
	p.mu.RUnlock()

	for _, addr := range peersToNotify {
		// multiple goroutines to notify peers concurrently
		go func(peerAddr string) {
			c, conn, err := dialPeer(peerAddr)
			if err != nil {
				fmt.Printf("Failed to connect to peer %s for notification: %v\n", peerAddr, err)
				return
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			c.NotifyJoin(ctx, &pb.PeerInfo{PeerId: p.id, PeerAddr: p.addr})
		}(addr)
	}

	return nil
}

func (p *PeerNode) leaveNetwork() {
	p.mu.RLock()
	peersToNotify := make(map[string]string, len(p.peers))
	for id, addr := range p.peers {
		peersToNotify[id] = addr
	}
	p.mu.RUnlock()

	var wg sync.WaitGroup
	for _, addr := range peersToNotify {
		wg.Add(1)
		go func(peerAddr string) {
			defer wg.Done()

			c, conn, err := dialPeer(peerAddr)
			if err != nil {
				fmt.Printf("Failed to connect to peer %s for leave notification: %v\n", peerAddr, err)
				return
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			c.Leave(ctx, &pb.LeaveRequest{PeerId: p.id, PeerAddr: p.addr})
		}(addr)
	}

	wg.Wait()
	fmt.Println("[*] Left the network gracefully.")
}

// Actions
func (p *PeerNode) sendTo(targetID, content string) {
	p.mu.RLock()
	addr, ok := p.peers[targetID]
	p.mu.RUnlock()

	if !ok {
		fmt.Printf("[!] Peer '%s' not found. Use 'list' to see known peers.\n", targetID)
		return
	}

	client, conn, err := dialPeer(addr)
	if err != nil {
		fmt.Printf("Failed to connect to peer %s: %v\n", targetID, err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.SendMessage(ctx, &pb.MessageRequest{
		SenderId:   p.id,
		SenderAddr: p.addr,
		Content:    content,
		Timestamp:  time.Now().Unix(),
	})
	if err != nil {
		fmt.Printf("Failed to send message to peer %s: %v\n", targetID, err)
		return
	}

	fmt.Printf("[*] Message sent to %s\n", targetID)
}

func (p *PeerNode) listPeers() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.peers) == 0 {
		fmt.Println("[*] No known peers yet.")
		return
	}

	fmt.Printf("[*] Known peers (%d): \n", len(p.peers))
	for id, addr := range p.peers {
		fmt.Printf("    %-15s @ %s\n", id, addr)
	}
}
