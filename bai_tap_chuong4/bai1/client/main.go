package main

import (
	pb "bai1/proto"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	useGzip := true
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // insecure credentials
	}
	if useGzip {
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	}

	conn, err := grpc.NewClient(
		"localhost:50052",
		dialOpts...,
	)
	if err != nil {
		log.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	client := pb.NewCalculatorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	/* resp, err := client.Add(ctx, &pb.AddRequest{A: 12, B: 30})
	if err != nil {
		log.Fatalf("Add RPC error: %v", err)
	}

	log.Printf("Result: %d", resp.Result) */ // Old Unary RPC

	stream, err := client.AddStream(ctx)
	if err != nil {
		log.Fatal(err)
	}

	inputs := []struct {
		a, b int32
	}{
		{a: 1, b: 2},
		{a: 10, b: 20},
		{a: 7, b: 8},
	}

	for _, in := range inputs {
		if err := stream.Send(&pb.AddRequest{A: in.a, B: in.b}); err != nil {
			log.Fatalf("Failed to send: %v", err)
		}
		log.Printf("Sent: %d + %d", in.a, in.b)

		resp, err := stream.Recv()
		if err != nil {
			log.Fatalf("Failed to receive: %v", err)
		}
		log.Printf("Received: %d", resp.Result)
	}

	_ = stream.CloseSend()
}
