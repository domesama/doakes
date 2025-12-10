package main

import (
	"log/slog"
	"time"

	"github.com/domesama/doakes/config"
	"github.com/domesama/doakes/server"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

func main() {
	// Create resource with service name
	res, err := resource.New(
		nil,
		resource.WithAttributes(attribute.String(string(semconv.ServiceNameKey), "my-service")),
	)
	if err != nil {
		panic("Failed to create resource")
	}

	// Create server with options
	srv, err := server.New(
		server.Options{
			Resource:       res,
			MetricsConfig:  config.DefaultMetricsConfig(),
			ServiceName:    "my-service",
			ServiceVersion: "1.0.0",
		},
	)
	if err != nil {
		panic("Failed to create server")
	}

	// Register custom health checks
	srv.RegisterHealthCheck(
		"database", func() error {
			// Check database connection
			return nil
		},
	)

	srv.RegisterHealthCheck(
		"cache", func() error {
			// Check cache connection
			return nil
		},
	)

	// Start server
	if err := srv.Start(); err != nil {
		panic("Failed to start server")
	}

	// Enable health checks after initialization
	srv.EnableHealthCheck()

	// Keep running
	slog.Info("TelemetryServer is running")
	time.Sleep(time.Hour)

	// Graceful shutdown
	if err := srv.Stop(); err != nil {
		slog.Info("Error during shutdown")
	}
}
