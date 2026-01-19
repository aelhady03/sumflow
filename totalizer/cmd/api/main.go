package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aelhady03/sumflow/totalizer/internal/database"
	"github.com/aelhady03/sumflow/totalizer/internal/dedup"
	"github.com/aelhady03/sumflow/totalizer/internal/kafka"
	"github.com/aelhady03/sumflow/totalizer/internal/service"
	"github.com/aelhady03/sumflow/totalizer/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

const version = "1.0.0"

type config struct {
	port         int
	env          string
	dbDSN        string
	kafkaBrokers string
	kafkaTopic   string
	kafkaGroupID string
}

type application struct {
	config   config
	logger   *slog.Logger
	service  *service.TotalizerService
	pool     *pgxpool.Pool
	consumer *kafka.Consumer
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 8080, "API Server Port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|production)")
	flag.StringVar(&cfg.dbDSN, "db-dsn", "postgres://totalizer:totalizer@localhost:5433/totalizer?sslmode=disable", "PostgreSQL DSN")
	flag.StringVar(&cfg.kafkaBrokers, "kafka-brokers", "kafka:9092", "Kafka broker addresses (comma-separated)")
	flag.StringVar(&cfg.kafkaTopic, "kafka-topic", "sums", "Kafka topic to consume")
	flag.StringVar(&cfg.kafkaGroupID, "kafka-group-id", "totalizer-group", "Kafka consumer group ID")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	pgStorage := storage.NewPostgresStorage(pool)
	dedupRepo := dedup.NewRepository(pool)

	// Initialize and start Kafka consumer
	consumerCfg := kafka.ConsumerConfig{
		Brokers: []string{cfg.kafkaBrokers},
		Topic:   cfg.kafkaTopic,
		GroupID: cfg.kafkaGroupID,
	}
	consumer := kafka.NewConsumer(consumerCfg, pool, dedupRepo, pgStorage)
	consumer.Start(ctx)

	// Initialize service
	svc := service.NewTotalizerService(pgStorage)

	app := &application{
		config:   cfg,
		logger:   logger,
		service:  svc,
		pool:     pool,
		consumer: consumer,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute * 1,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down gracefully...")

		cancel()
		if err := app.consumer.Stop(); err != nil {
			logger.Error("error stopping consumer", slog.String("error", err.Error()))
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down server", slog.String("error", err.Error()))
		}

		logger.Info("shutdown complete")
	}()

	logger.Info("starting server", slog.String("addr", srv.Addr), slog.String("env", app.config.env))

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
