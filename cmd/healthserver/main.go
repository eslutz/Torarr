package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eslutz/torarr/internal/config"
	"github.com/eslutz/torarr/internal/health"
)

func main() {
	// Setup JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	handler := health.NewHandler(cfg)
	defer func() {
		if err := handler.Close(); err != nil {
			slog.Error("Failed to close handler", "error", err)
		}
	}()

	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.HealthPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("Starting health server", "port", cfg.HealthPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	fmt.Println("Server stopped")
}
