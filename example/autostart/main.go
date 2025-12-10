package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/domesama/doakes/doakeswire"
)

func main() {
	// Initialize server with Wire - auto-starts and returns cleanup function
	srv, cleanup, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	if err != nil {
		panic("Failed to initialize server")
	}
	defer cleanup()

	// Register health checks
	srv.RegisterHealthCheck("database", checkDatabase)
	srv.RegisterHealthCheck("cache", checkCache)

	// Enable health checks after initialization is complete
	srv.EnableHealthCheck()

	slog.Info("Server is running and ready")

	// Wait for shutdown signal
	waitForShutdown()

	slog.Info("Shutting down gracefully")
}

func checkDatabase() error {
	// Your database health check logic
	return nil
}

func checkCache() error {
	// Your cache health check logic
	return nil
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}
