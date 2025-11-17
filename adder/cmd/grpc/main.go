package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/aelhady03/sumflow/adder/internal/server"
	sumpb "github.com/aelhady03/sumflow/adder/proto/sum"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := flag.Int("port", 50051, "gRPC Server Port")

	li, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to connect to gRPC server on port %d", *port)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	sumpb.RegisterSumNumbersServiceServer(grpcServer, server.NewSumNumbersServer())

	log.Printf("gRPC server started at port %d\n", *port)

	if err := grpcServer.Serve(li); err != nil {
		log.Fatalf("failed to serve gRPC server on port %d", *port)
	}
}
