package main

import (
	pb "bai1/proto"
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	useGzip := true

	tlsCreds, err := credentials.NewClientTLSFromFile("certs/ca.crt", "localhost")
	if err != nil {
		log.Fatalf("Failed to create TLS credentials: %v", err)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
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

	/* ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel() */

	// add authentication metadata
	ctx := context.Background()
	md := metadata.Pairs("x-api-key", "secret-api-key")
	ctx = metadata.NewOutgoingContext(ctx, md)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	inputs := []struct {
		a, b int32
	}{
		{a: 10, b: 5},
		{a: 20, b: 4},
		{a: 100, b: 10},
	}

	// Test AddStream
	fmt.Println("\n========== Testing AddStream ==========")
	testAddStream(client, ctx, inputs)

	// Test SubStream
	fmt.Println("\n========== Testing SubStream ==========")
	testSubStream(client, ctx, inputs)

	// Test MulStream
	fmt.Println("\n========== Testing MulStream ==========")
	testMulStream(client, ctx, inputs)

	// Test DivStream
	fmt.Println("\n========== Testing DivStream ==========")
	testDivStream(client, ctx, inputs)

	// Test DivStream with division by zero
	fmt.Println("\n========== Testing DivStream (Division by Zero) ==========")
	testDivStream(client, ctx, []struct{ a, b int32 }{{a: 10, b: 0}})
}

func testAddStream(client pb.CalculatorServiceClient, ctx context.Context, inputs []struct{ a, b int32 }) {
	stream, err := client.AddStream(ctx)
	if err != nil {
		handleError("open AddStream", err)
		return
	}

	for _, in := range inputs {
		if err := stream.Send(&pb.Request{A: in.a, B: in.b}); err != nil {
			handleError("send AddStream request", err)
			return
		}
		log.Printf("Sent: %d + %d", in.a, in.b)

		resp, err := stream.Recv()
		if err != nil {
			handleError("receive AddStream response", err)
			return
		}
		log.Printf("✓ Received: %d", resp.Result)
	}

	if err := stream.CloseSend(); err != nil {
		handleError("close AddStream", err)
		return
	}
}

func testSubStream(client pb.CalculatorServiceClient, ctx context.Context, inputs []struct{ a, b int32 }) {
	stream, err := client.SubStream(ctx)
	if err != nil {
		handleError("open SubStream", err)
		return
	}

	for _, in := range inputs {
		if err := stream.Send(&pb.Request{A: in.a, B: in.b}); err != nil {
			handleError("send SubStream request", err)
			return
		}
		log.Printf("Sent: %d - %d", in.a, in.b)

		resp, err := stream.Recv()
		if err != nil {
			handleError("receive SubStream response", err)
			return
		}
		log.Printf("✓ Received: %d", resp.Result)
	}

	if err := stream.CloseSend(); err != nil {
		handleError("close SubStream", err)
		return
	}
}

func testMulStream(client pb.CalculatorServiceClient, ctx context.Context, inputs []struct{ a, b int32 }) {
	stream, err := client.MulStream(ctx)
	if err != nil {
		handleError("open MulStream", err)
		return
	}

	for _, in := range inputs {
		if err := stream.Send(&pb.Request{A: in.a, B: in.b}); err != nil {
			handleError("send MulStream request", err)
			return
		}
		log.Printf("Sent: %d * %d", in.a, in.b)

		resp, err := stream.Recv()
		if err != nil {
			handleError("receive MulStream response", err)
			return
		}
		log.Printf("✓ Received: %d", resp.Result)
	}

	if err := stream.CloseSend(); err != nil {
		handleError("close MulStream", err)
		return
	}
}

func testDivStream(client pb.CalculatorServiceClient, ctx context.Context, inputs []struct{ a, b int32 }) {
	stream, err := client.DivStream(ctx)
	if err != nil {
		handleError("open DivStream", err)
		return
	}

	for _, in := range inputs {
		if err := stream.Send(&pb.Request{A: in.a, B: in.b}); err != nil {
			handleError("send DivStream request", err)
			return
		}
		log.Printf("Sent: %d / %d", in.a, in.b)

		resp, err := stream.Recv()
		if err != nil {
			handleError("receive DivStream response", err)
			return
		}
		log.Printf("✓ Received: %d", resp.Result)
	}

	if err := stream.CloseSend(); err != nil {
		handleError("close DivStream", err)
		return
	}
}

func handleError(operation string, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Printf("✗ %s failed (unknown error): %v", operation, err)
		return
	}
	switch st.Code() {
	case codes.DeadlineExceeded:
		log.Printf("✗ %s failed: TIMEOUT (server did not respond within the allowed time)", operation)
		log.Printf("  → Solution: increase timeout or check server performance")

	case codes.Unauthenticated:
		log.Printf("✗ %s failed: AUTHENTICATION ERROR (%s)", operation, st.Message())
		log.Printf("  → Solution: check API key or credentials")

	case codes.Unavailable:
		log.Printf("✗ %s failed: SERVER UNAVAILABLE (server is down or network error)", operation)
		log.Printf("  → Solution: retry after a few seconds or check server status")

	case codes.Canceled:
		log.Printf("✗ %s failed: REQUEST CANCELED (context was canceled)", operation)
		log.Printf("  → Solution: check cancel logic in code")

	case codes.Internal:
		log.Printf("✗ %s failed: SERVER INTERNAL ERROR (%s)", operation, st.Message())
		log.Printf("  → Solution: check server logs for debugging")

	case codes.InvalidArgument:
		log.Printf("✗ %s failed: INVALID ARGUMENT (%s)", operation, st.Message())
		log.Printf("  → Solution: check input data")

	default:
		log.Printf("✗ %s failed: %s (%s)", operation, st.Code(), st.Message())
	}
}
