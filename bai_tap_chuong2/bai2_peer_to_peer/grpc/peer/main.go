package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	pb "p2p_grpc/proto"
	"strings"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:  peer <peer-id> <port> [bootstrap-addr]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run peer/main.go Alice :9001")
		fmt.Println("  go run peer/main.go Bob   :9002 localhost:9001")
		fmt.Println("  go run peer/main.go Carol :9003 localhost:9001")
		os.Exit(1)
	}

	peerID := os.Args[1]
	port := os.Args[2]
	selfAddr := "localhost" + port

	node := NewPeerNode(peerID, selfAddr)

	// Start gRPC server
	lis, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("Failed to listen on %s: %v\n", port, err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPeerServiceServer(grpcServer, node)

	go func() {
		fmt.Printf("[*] Peer '%s' started on %s\n", peerID, selfAddr)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// If bootstrap address provided, join the network
	if len(os.Args) >= 4 {
		bootstrapAddr := os.Args[3]
		fmt.Printf("[*] Joining network via %s...\n", bootstrapAddr)
		if err := node.joinNetwork(bootstrapAddr); err != nil {
			fmt.Printf("[!] %v\n", err)
		}
	} else {
		fmt.Println("[*] Started as first peer (no bootstrap).")
	}

	// ── Interactive CLI ──────────────────────────────────────
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  send <peer-id> <message>  — send a message directly to a peer")
	fmt.Println("  list                      — list all known peers")
	fmt.Println("  quit                      — leave network and exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		switch parts[0] {
		case "send":
			if len(parts) < 3 {
				fmt.Println("Usage: send <peer-id> <message>")
				continue
			}
			node.sendTo(parts[1], parts[2])

		case "list":
			node.listPeers()

		case "quit":
			fmt.Println("[*] Leaving network...")
			node.leaveNetwork()
			grpcServer.GracefulStop()
			os.Exit(0)

		default:
			fmt.Printf("[!] Unknown command: '%s'\n", parts[0])
		}
	}
}
