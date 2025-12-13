# Meter Usage Guide

This document explains how to use OpenTelemetry meters in the Doakes library.

## Overview

Doakes automatically sets up a global OpenTelemetry `MeterProvider` during initialization. This allows you to create and use metrics anywhere in your application without manually passing around provider instances.

## Quick Start

### 1. Initialize Doakes Server

```go
import "github.com/domesama/doakes/doakeswire"

func main() {
    // This automatically sets up the global meter provider
    srv, cleanup, err := doakeswire.InitializeTelemetryServerWithAutoStart()
    if err != nil {
        panic(err)
    }
    defer cleanup()
    
    srv.EnableHealthCheck()
    
    // Your application code...
}
```

### 2. Get a Meter

There are three ways to get a meter:

#### Option A: Using `doakeswire.GetMeter()` (Recommended)

The simplest way - automatically uses your `OTEL_SERVICE_NAME` as the meter scope:

```go
import "github.com/domesama/doakes/doakeswire"

meter := doakeswire.GetMeter()
```

#### Option B: Using `metrics.GetDefaultMeter()`

Same as Option A, but from the metrics package:

```go
import "github.com/domesama/doakes/metrics"

meter := metrics.GetDefaultMeter()
```

#### Option C: Using `otel.Meter()` Directly

For custom scope names:

```go
import "go.opentelemetry.io/otel"

meter := otel.Meter("my-custom-scope")
```

### 3. Create Metrics

Once you have a meter, create and use metrics:

```go
import (
    "context"
    "github.com/domesama/doakes/doakeswire"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

func main() {
    // ... server initialization ...
    
    meter := doakeswire.GetMeter()
    
    // Create a counter
    requestCounter, _ := meter.Int64Counter("http_requests_total")
    requestCounter.Add(context.Background(), 1, 
        metric.WithAttributes(
            attribute.String("method", "GET"),
            attribute.String("path", "/api/users"),
        ))
    
    // Create a histogram
    latencyHistogram, _ := meter.Int64Histogram("request_duration_ms")
    latencyHistogram.Record(context.Background(), 150,
        metric.WithAttributes(attribute.String("endpoint", "/api")))
    
    // Create a gauge (via observable)
    meter.Int64ObservableGauge("active_connections",
        metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
            o.Observe(getCurrentConnections())
            return nil
        }))
}
```

## How It Works

1. **During Initialization**: When you create a Doakes server, the `metrics.NewProvider()` function creates an OpenTelemetry `MeterProvider` configured with a Prometheus exporter.

2. **Global Registration**: The provider calls `otel.SetMeterProvider()` to register itself globally.

3. **Meter Retrieval**: When you call `doakeswire.GetMeter()` or `otel.Meter()`, it gets the meter from this global provider.

4. **Automatic Export**: All metrics created through these meters are automatically collected and exposed at `http://localhost:28080/metrics` in Prometheus format.

## Service Name Configuration

The default meter scope name comes from the `OTEL_SERVICE_NAME` environment variable:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_SERVICE_VERSION="1.0.0"
```

If `OTEL_SERVICE_NAME` is not set, Doakes will use `"unknown-service"` as the default.

## Best Practices

### 1. Use the Helper Function

```go
// ✅ Recommended - automatically scoped to service name
meter := doakeswire.GetMeter()

// ❌ Not recommended - requires manual service name
meter := otel.Meter("my-service")
```

### 2. Create Instruments Once

Instruments (counters, histograms, gauges) should be created once and reused:

```go
// ✅ Good - create once
var requestCounter metric.Int64Counter

func init() {
    meter := doakeswire.GetMeter()
    requestCounter, _ = meter.Int64Counter("requests_total")
}

func handler() {
    requestCounter.Add(context.Background(), 1)
}

// ❌ Bad - creates a new instrument on every call
func handler() {
    meter := doakeswire.GetMeter()
    counter, _ := meter.Int64Counter("requests_total")
    counter.Add(context.Background(), 1)
}
```

### 3. Use Attributes for Dimensions

```go
counter.Add(ctx, 1, metric.WithAttributes(
    attribute.String("method", "GET"),
    attribute.String("status", "200"),
    attribute.String("path", "/api/users"),
))
```

### 4. Choose the Right Metric Type

- **Counter**: Monotonically increasing values (requests, errors)
- **Histogram**: Distribution of values (latency, response size)
- **Gauge**: Point-in-time values (connections, queue size)

## Complete Example

```go
package main

import (
    "context"
    "log/slog"
    "os"
    
    "github.com/domesama/doakes/doakeswire"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    requestCounter   metric.Int64Counter
    latencyHistogram metric.Int64Histogram
)

func main() {
    // Set required env vars
    os.Setenv("OTEL_SERVICE_NAME", "my-api")
    os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
    
    // Initialize server
    srv, cleanup, err := doakeswire.InitializeTelemetryServerWithAutoStart()
    if err != nil {
        panic(err)
    }
    defer cleanup()
    
    srv.EnableHealthCheck()
    
    // Initialize metrics
    setupMetrics()
    
    // Your application logic
    handleRequest()
    
    slog.Info("Metrics available at http://localhost:28080/metrics")
}

func setupMetrics() {
    meter := doakeswire.GetMeter()
    
    requestCounter, _ = meter.Int64Counter("api_requests_total",
        metric.WithDescription("Total number of API requests"))
    
    latencyHistogram, _ = meter.Int64Histogram("api_request_duration_ms",
        metric.WithDescription("API request duration in milliseconds"))
    
    meter.Int64ObservableGauge("goroutines",
        metric.WithDescription("Number of goroutines"),
        metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
            o.Observe(int64(runtime.NumGoroutine()))
            return nil
        }))
}

func handleRequest() {
    ctx := context.Background()
    
    // Record request
    requestCounter.Add(ctx, 1, metric.WithAttributes(
        attribute.String("method", "GET"),
        attribute.String("endpoint", "/api/users"),
    ))
    
    // Record latency
    latencyHistogram.Record(ctx, 42, metric.WithAttributes(
        attribute.String("endpoint", "/api/users"),
    ))
}
```

## Troubleshooting

### Metrics not appearing at /metrics endpoint

1. Ensure the Doakes server is started
2. Check that you're creating metrics AFTER server initialization
3. Verify `OTEL_SERVICE_NAME` is set

### Getting "unknown-service" in metrics

Set the `OTEL_SERVICE_NAME` environment variable:

```bash
export OTEL_SERVICE_NAME="my-service"
```

### Histogram buckets not as expected

Configure custom histogram boundaries in `config.MetricsConfig` before creating the provider.

## Related Documentation

- [OpenTelemetry Metrics API](https://opentelemetry.io/docs/instrumentation/go/manual/#metrics)
- [Doakes README](./README.md)
- [Prometheus Exposition Format](https://prometheus.io/docs/instrumenting/exposition_formats/)

