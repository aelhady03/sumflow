package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aelhady03/sumflow/adder/internal/kafka"
	"github.com/aelhady03/sumflow/adder/internal/server"
	"github.com/aelhady03/sumflow/adder/internal/service"
	sumpb "github.com/aelhady03/sumflow/adder/proto/sum"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type config struct {
	port         int
	kafkaBrokers string
	kafkaTopic   string
}

type application struct {
	config     config
	grpcServer *grpc.Server
	service    *service.AdderService
	producer   *kafka.KafkaProducer
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 50051, "gRPC Server Port")
	flag.StringVar(&cfg.kafkaBrokers, "kafka-brokers", "kafka:9092", "Kafka broker addresses (comma-separated)")
	flag.StringVar(&cfg.kafkaTopic, "kafka-topic", "sums", "Kafka topic name")
	flag.Parse()

	li, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.port))
	if err != nil {
		log.Fatalf("failed to connect to gRPC server on port %d", cfg.port)
	}

	kafkaProducer := kafka.NewKafkaProducer([]string{cfg.kafkaBrokers}, cfg.kafkaTopic)

	grpcServer := grpc.NewServer()
	adderSvc := service.NewAdderService(kafkaProducer)

	app := &application{
		config:     cfg,
		grpcServer: grpcServer,
		service:    adderSvc,
		producer:   kafkaProducer,
	}

	reflection.Register(app.grpcServer)
	sumpb.RegisterSumNumbersServiceServer(app.grpcServer, server.NewSumNumbersServer(app.service))

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")

		app.grpcServer.GracefulStop()

		if err := app.producer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
		}

		log.Println("Shutdown complete")
	}()

	log.Printf("gRPC server started at port %d\n", app.config.port)

	if err := app.grpcServer.Serve(li); err != nil {
		log.Fatalf("failed to serve gRPC server on port %d", app.config.port)
	}
}
