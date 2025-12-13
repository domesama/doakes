package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/domesama/doakes/doakeswire"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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

	// Get meter scoped to the service name from OTEL_SERVICE_NAME
	meter := doakeswire.GetMeter()

	// Create some example metrics
	counter, _ := meter.Int64Counter("example_requests_total")
	counter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", "GET")))

	histogram, _ := meter.Int64Histogram("example_request_duration_ms")
	histogram.Record(context.Background(), 150, metric.WithAttributes(attribute.String("endpoint", "/api")))

	// Get the actual running port (useful when using :0 for dynamic port)
	port := srv.GetRunningPort()
	addr := srv.GetRunningAddress()

	slog.Info(
		"Server is running and ready",
		"port", port,
		"address", addr,
		"metrics_url", fmt.Sprintf("http://localhost:%d/metrics", port),
		"health_url", fmt.Sprintf("http://localhost:%d/_hc", port),
	)

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
