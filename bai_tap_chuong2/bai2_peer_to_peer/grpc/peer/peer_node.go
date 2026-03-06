package main

import (
	pb "p2p_grpc/proto"
	"sync"
)

/*  --- PeerNode ---  */
type PeerNode struct {
	pb.UnimplementedPeerServiceServer
	id    string
	addr  string            // "localhost:port"
	peers map[string]string // peerID -> peerAddr
	mu    sync.RWMutex      // lock
}

func NewPeerNode(id, adds string) *PeerNode {
	return &PeerNode{
		id:    id,
		addr:  adds,
		peers: make(map[string]string),
	}
}
