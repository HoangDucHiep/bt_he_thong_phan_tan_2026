package main

import (
	pb "bai1/proto"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const validAPIKey = "secret-api-key"

// stub
type calculatorServer struct {
	pb.UnimplementedCalculatorServiceServer
}

/* func (s *calculatorServer) Add(ctx context.Context, req *pb.AddRequest) (*pb.AddResponse, error) {
	return &pb.AddResponse{Result: req.A + req.B}, nil
} */

func (s *calculatorServer) AddStream(stream pb.CalculatorService_AddStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// fake error for testing
		if req.A == 999 {
			return status.Error(codes.Internal, "simulated server error for testing")
		}

		if req.A == 888 {
			log.Println("Simulating slow server (10s delay)...")
			time.Sleep(10 * time.Second)
		}

		resp := &pb.Response{Result: req.A + req.B}

		if err = stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *calculatorServer) SubStream(stream pb.CalculatorService_SubStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := &pb.Response{Result: req.A - req.B}

		if err = stream.Send(resp); err != nil {
			return err
		}

	}
}

func (s *calculatorServer) MulStream(stream pb.CalculatorService_MulStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := &pb.Response{Result: req.A * req.B}

		if err = stream.Send(resp); err != nil {
			return err
		}

	}
}

func (s *calculatorServer) DivStream(stream pb.CalculatorService_DivStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if req.B == 0 {
			errMsg := fmt.Sprintf("division by zero: %d / %d", req.A, req.B)
			return status.Error(codes.InvalidArgument, errMsg)
		}

		resp := &pb.Response{Result: req.A / req.B}
		if err = stream.Send(resp); err != nil {
			return err
		}
	}
}

// Stream interceptor for API key authentication
func streamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())

	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md["x-api-key"]
	if len(apiKeys) == 0 {
		return status.Error(codes.Unauthenticated, "missing API key")
	}
	if apiKeys[0] != validAPIKey {
		return status.Error(codes.Unauthenticated, "invalid API key")
	}
	return handler(srv, ss)
}

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile("certs/server.crt", "certs/server.key")
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(creds),
		grpc.StreamInterceptor(streamAuthInterceptor),
	)

	pb.RegisterCalculatorServiceServer(grpcServer, &calculatorServer{})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	fmt.Println("Server is running on port 50052")

}
