package main

import (
	pb "bai1/proto"
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
)

type calculatorServer struct {
	pb.UnimplementedCalculatorServiceServer
}

func (s *calculatorServer) Add(ctx context.Context, req *pb.AddRequest) (*pb.AddResponse, error) {
	return &pb.AddResponse{Result: req.A + req.B}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterCalculatorServiceServer(grpcServer, &calculatorServer{})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	fmt.Println("Server is running on port 50052")

}
