package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aelhady03/sumflow/adder/internal/database"
	"github.com/aelhady03/sumflow/adder/internal/kafka"
	"github.com/aelhady03/sumflow/adder/internal/outbox"
	"github.com/aelhady03/sumflow/adder/internal/server"
	"github.com/aelhady03/sumflow/adder/internal/service"
	"github.com/aelhady03/sumflow/pkg/telemetry"
	sumpb "github.com/aelhady03/sumflow/adder/proto/sum"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type config struct {
	port          int
	metricsPort   int
	dbDSN         string
	kafkaBrokers  string
	kafkaTopic    string
	relayInterval time.Duration
	relayBatch    int
	otlpEndpoint  string
}

type application struct {
	config     config
	grpcServer *grpc.Server
	service    *service.AdderService
	producer   *kafka.KafkaProducer
	pool       *pgxpool.Pool
	relay      *outbox.Relay
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 50051, "gRPC Server Port")
	flag.IntVar(&cfg.metricsPort, "metrics-port", 9090, "Prometheus metrics port")
	flag.StringVar(&cfg.dbDSN, "db-dsn", "postgres://adder:adder@localhost:5432/adder?sslmode=disable", "PostgreSQL DSN")
	flag.StringVar(&cfg.kafkaBrokers, "kafka-brokers", "kafka:9092", "Kafka broker addresses (comma-separated)")
	flag.StringVar(&cfg.kafkaTopic, "kafka-topic", "sums", "Kafka topic name")
	flag.DurationVar(&cfg.relayInterval, "relay-interval", 100*time.Millisecond, "Outbox relay polling interval")
	flag.IntVar(&cfg.relayBatch, "relay-batch", 100, "Outbox relay batch size")
	flag.StringVar(&cfg.otlpEndpoint, "otlp-endpoint", "otel-collector:4317", "OpenTelemetry Collector endpoint")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize telemetry
	shutdownTracer, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName:    "adder",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   cfg.otlpEndpoint,
	})
	if err != nil {
		log.Printf("Warning: failed to initialize tracer: %v", err)
	}

	// Initialize database
	dbConfig := database.DefaultConfig(cfg.dbDSN)
	pool, err := database.NewPool(ctx, dbConfig)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Initialize components
	outboxRepo := outbox.NewRepository(pool)
	kafkaProducer := kafka.NewKafkaProducer([]string{cfg.kafkaBrokers}, cfg.kafkaTopic)

	// Configure and start relay
	relayConfig := outbox.DefaultRelayConfig()
	relayConfig.PollInterval = cfg.relayInterval
	relayConfig.BatchSize = cfg.relayBatch
	relay := outbox.NewRelay(outboxRepo, kafkaProducer, relayConfig)
	relay.Start(ctx)

	// Initialize service and server with OTel interceptors
	adderSvc := service.NewAdderService(pool, outboxRepo)
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	li, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.port))
	if err != nil {
		log.Fatalf("failed to listen on port %d: %v", cfg.port, err)
	}

	app := &application{
		config:     cfg,
		grpcServer: grpcServer,
		service:    adderSvc,
		producer:   kafkaProducer,
		pool:       pool,
		relay:      relay,
	}

	reflection.Register(app.grpcServer)
	sumpb.RegisterSumNumbersServiceServer(app.grpcServer, server.NewSumNumbersServer(app.service))

	// Start metrics server
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.metricsPort),
		Handler: promhttp.Handler(),
	}
	go func() {
		log.Printf("Metrics server started at port %d\n", cfg.metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")

		cancel()
		app.relay.Stop()
		app.grpcServer.GracefulStop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down metrics server: %v", err)
		}

		if shutdownTracer != nil {
			if err := shutdownTracer(shutdownCtx); err != nil {
				log.Printf("Error shutting down tracer: %v", err)
			}
		}

		if err := app.producer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
		}

		log.Println("Shutdown complete")
	}()

	log.Printf("gRPC server started at port %d\n", app.config.port)

	if err := app.grpcServer.Serve(li); err != nil {
		log.Fatalf("failed to serve gRPC server on port %d: %v", app.config.port, err)
	}
}
