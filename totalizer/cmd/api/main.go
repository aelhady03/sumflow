package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aelhady03/sumflow/totalizer/internal/service"
	"github.com/aelhady03/sumflow/totalizer/internal/storage"
)

const version = "1.0.0"

type config struct {
	port    int
	env     string
	storage struct {
		filename string
	}
}

type application struct {
	config  config
	logger  *slog.Logger
	service *service.TotalizerService
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 8080, "API Server Port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|production)")
	flag.StringVar(&cfg.storage.filename, "filename", "data.txt", "Filename to Load/Save total count")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	storage := storage.NewFileStorage(cfg.storage.filename)
	service := service.NewTotalizerService(storage)

	app := &application{
		config:  cfg,
		logger:  logger,
		service: service,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute * 1,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", slog.String("addr", srv.Addr), slog.String("env", app.config.env), slog.String("filename", app.config.storage.filename))

	err := srv.ListenAndServe()

	logger.Error(err.Error())

	os.Exit(1)
}
