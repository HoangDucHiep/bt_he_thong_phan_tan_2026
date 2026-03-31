package main

import (
	pb "bai1/proto"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"localhost:50052",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	client := pb.NewCalculatorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.Add(ctx, &pb.AddRequest{A: 12, B: 30})
	if err != nil {
		log.Fatalf("Add RPC error: %v", err)
	}

	log.Printf("Result: %d", resp.Result)
}
