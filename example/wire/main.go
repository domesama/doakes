package main

import (
	"log/slog"
	"time"

	"github.com/domesama/doakes/doakeswire"
)

func main() {
	// Initialize server with wire
	srv, err := doakeswire.InitializeTelemetryServer()
	if err != nil {
		panic("Failed to initialize server")
	}

	// Register custom health checks
	srv.RegisterHealthCheck(
		"api_ready", func() error {
			// Check if API is ready
			return nil
		},
	)

	// Start server
	if err := srv.Start(); err != nil {
		slog.Error("Failed to start server")
	}

	// Enable health checks
	srv.EnableHealthCheck()

	// Keep running
	slog.Info("Server running with wire")
	time.Sleep(time.Hour)

	// Shutdown
	if err := srv.Stop(); err != nil {
		slog.Error("Error during shutdown")
	}
}
