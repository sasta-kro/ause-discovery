package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ause-discovery.local/backend/internal/platform/config"
	"ause-discovery.local/backend/internal/platform/httpserver"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	configuration, err := config.Load(os.Getenv)
	if err != nil {
		slog.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	logger := newLogger(configuration)
	if err := runAPI(configuration, logger); err != nil {
		logger.Error("api stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func runAPI(configuration config.Config, logger *slog.Logger) error {
	if err := configuration.EnsureStorageDirectories(); err != nil {
		return err
	}

	applicationContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databasePool, err := pgxpool.New(applicationContext, configuration.DatabaseURL)
	if err != nil {
		return err
	}
	defer databasePool.Close()

	if err := databasePool.Ping(applicationContext); err != nil {
		return err
	}

	mux := http.NewServeMux()
	httpserver.HealthHandler{DatabasePool: databasePool}.Register(mux, configuration.PublicBasePath)

	server := &http.Server{
		Addr:              configuration.ListenAddress,
		Handler:           requestLogger(logger, mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	logger.Info("api started", "address", configuration.ListenAddress, "public_base_path", configuration.PublicBasePath)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-applicationContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}

func runHealthcheck() int {
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return 1
	}

	context, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	databasePool, err := pgxpool.New(context, configuration.DatabaseURL)
	if err != nil {
		return 1
	}
	defer databasePool.Close()

	if err := databasePool.Ping(context); err != nil {
		return 1
	}

	return 0
}

func newLogger(configuration config.Config) *slog.Logger {
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(configuration.LogLevel)); err != nil {
		level.Set(slog.LevelInfo)
	}

	if configuration.Environment == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(responseWriter, request)
		logger.Info("request completed", "method", request.Method, "path", request.URL.Path, "duration", time.Since(startedAt))
	})
}
