# Doakes

A Go library that provides an internal telemetry server for exposing metrics, health checks, and profiling endpoints using OpenTelemetry and Prometheus.

![Doakes](https://media3.giphy.com/media/v1.Y2lkPTc5MGI3NjExOGdyamtpczJ2eWFhMjZpOTdsNG1zOHRwcWNvZ2pldDBlZm00dzI2eSZlcD12MV9pbnRlcm5hbF9naWZfYnlfaWQmY3Q9Zw/iuu3hRoxlr2ETPucZW/giphy.gif)

## What is it?

Doakes is a batteries-included observability solution for Go services. It provides:

- **Metrics**: OpenTelemetry metrics exported via Prometheus format
- **Health Checks**: Customizable health check endpoints for orchestration platforms
- **Profiling**: Built-in pprof endpoints for performance debugging
- **Runtime Metrics**: Automatic Go runtime metrics (memory, goroutines, GC stats)

The library runs an internal HTTP server (default port `:28080`) that exposes these observability endpoints, keeping them separate from your main application server.

## Getting Started

### Installation

```bash
go get github.com/domesama/doakes
```

### Quick Start

#### Option 1: Using Wire (Recommended)

If you use [Google Wire](https://github.com/google/wire) for dependency injection:

```go
package main

import (
    "log/slog"
    "os"
    
    "github.com/domesama/doakes/wire"
)

func main() {
    // Set required environment variables
    os.Setenv("OTEL_SERVICE_NAME", "my-service")
    os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
    
    // Initialize with auto-start
    srv, cleanup, err := wire.InitializeTelemetryServerWithAutoStart()
    if err != nil {
        panic(err)
    }
    defer cleanup()
    
    // Register health checks
    srv.RegisterHealthCheck("database", checkDatabase)
    
    // Enable health checks AFTER initialization is complete
    srv.EnableHealthCheck()
    
    // Your application logic here...
    runApplication()
}

func checkDatabase() error {
    // Your database health check logic
    return nil
}
```

#### Option 2: Manual Setup

```go
package main

import (
    "github.com/domesama/doakes/config"
    "github.com/domesama/doakes/server"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

func main() {
    // Create resource
    res, err := resource.New(
        nil,
        resource.WithAttributes(
            semconv.ServiceNameKey.String("my-service"),
            semconv.ServiceVersionKey.String("1.0.0"),
        ),
    )
    if err != nil {
        panic(err)
    }
    
    // Create server
    srv, err := server.New(server.Options{
        Resource:       res,
        MetricsConfig:  config.DefaultMetricsConfig(),
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
    })
    if err != nil {
        panic(err)
    }
    
    // Register health checks
    srv.RegisterHealthCheck("database", checkDatabase)
    
    // Start server
    if err := srv.Start(); err != nil {
        panic(err)
    }
    defer srv.Stop()
    
    // Enable health checks
    srv.EnableHealthCheck()
    
    // Your application logic here...
}
```

## How to Wire Using TelemetrySet

Doakes provides two Wire provider sets for easy integration:

### 1. TelemetrySet (Manual Start)

Use when you want full control over server lifecycle:

```go
//go:build wireinject
//+build wireinject

package main

import (
    "github.com/domesama/doakes/server"
    "github.com/domesama/doakes/wire"
    "github.com/google/wire"
)

func InitializeServer() (*server.TelemetryServer, error) {
    wire.Build(
        wire.TelemetrySet,
        // Your other providers...
    )
    return nil, nil
}
```

Then in your main:

```go
srv, err := InitializeServer()
if err != nil {
    panic(err)
}

if err := srv.Start(); err != nil {
    panic(err)
}
defer srv.Stop()

srv.EnableHealthCheck()
```

### 2. TelemetrySetWithAutoStart (Recommended)

Use for simpler setup - server starts automatically:

```go
//go:build wireinject
//+build wireinject

package main

import (
    "github.com/domesama/doakes/server"
    "github.com/domesama/doakes/wire"
    "github.com/google/wire"
)

func InitializeServerWithAutoStart() (*server.TelemetryServer, func(), error) {
    wire.Build(wire.TelemetrySetWithAutoStart)
    return nil, nil, nil
}
```

Then in your main:

```go
srv, cleanup, err := InitializeServerWithAutoStart()
if err != nil {
    panic(err)
}
defer cleanup() // Automatically stops server

srv.EnableHealthCheck()
```

### Wire Providers Included

The `TelemetrySet` provides:

- `ProvideResource()` - Creates OpenTelemetry resource from environment variables
- `ProvideMetricsConfig()` - Returns default metrics configuration
- `ProvideServerOptions()` - Builds server options from dependencies
- `server.New()` - Creates the TelemetryServer instance

## When and Why Health Checks Need to be Called

### The Health Check Pattern

Health checks serve a critical purpose in orchestrated environments (Kubernetes, ECS, etc.):

1. **Liveness**: Is the service alive?
2. **Readiness**: Is the service ready to accept traffic?

### Why EnableHealthCheck() Must Be Called Explicitly

**Problem**: If health checks are enabled by default, orchestrators will route traffic to services that are still initializing - connecting to databases, warming caches, loading configuration, etc.

**Solution**: This library requires you to explicitly call `EnableHealthCheck()` AFTER your initialization is complete. This ensures:

- Services don't receive traffic before they're truly ready
- You consciously decide when your service is ready
- Orchestrators only route to fully initialized instances

### The Timeout Mechanism

To prevent accidentally forgetting to enable health checks, the server monitors for `EnableHealthCheck()` calls:

```go
// Default timeout: 1 minute
// Set via: INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION=1m

srv.EnableHealthCheck() // Must be called within timeout
```

If `EnableHealthCheck()` is not called within the timeout, **the server will panic** to fail fast. This is intentional - better to crash during startup than silently accept traffic before being ready.

### Example: Proper Initialization Flow

```go
func main() {
    // 1. Start telemetry server
    srv, cleanup, err := wire.InitializeTelemetryServerWithAutoStart()
    if err != nil {
        panic(err)
    }
    defer cleanup()
    
    // 2. Register health checks
    srv.RegisterHealthCheck("database", checkDatabase)
    srv.RegisterHealthCheck("cache", checkCache)
    
    // 3. Initialize your application components
    db := connectDatabase()
    cache := initializeCache()
    
    // 4. NOW enable health checks - initialization complete
    srv.EnableHealthCheck()
    
    // 5. Start accepting traffic
    startMainServer()
}
```

### Health Check States

- **Before EnableHealthCheck()**: Endpoint returns `503 Service Unavailable`
- **After EnableHealthCheck()**: Endpoint returns `200 OK` (if all checks pass)

## What You Can Do with TelemetryServer

### 1. Register Custom Health Checks

```go
srv.RegisterHealthCheck("database", func() error {
    if err := db.Ping(); err != nil {
        return fmt.Errorf("database unreachable: %w", err)
    }
    return nil
})

srv.RegisterHealthCheck("cache", func() error {
    if !cache.IsConnected() {
        return errors.New("cache not connected")
    }
    return nil
})

srv.RegisterHealthCheck("external-api", func() error {
    resp, err := http.Get("https://api.example.com/health")
    if err != nil || resp.StatusCode != 200 {
        return errors.New("external API unhealthy")
    }
    return nil
})
```

### 2. Use OpenTelemetry Metrics

The server automatically sets up a global meter provider. You can create metrics in two ways:

#### Option A: Use the GetMeter() Helper (Recommended)

```go
import (
    "github.com/domesama/doakes/doakeswire"
    "go.opentelemetry.io/otel/attribute"
)

// Get meter automatically scoped to your OTEL_SERVICE_NAME
meter := doakeswire.GetMeter()

// Create counter
requestCounter, _ := meter.Int64Counter("http_requests_total")
requestCounter.Add(ctx, 1, attribute.String("method", "GET"))

// Create histogram
latencyHistogram, _ := meter.Int64Histogram("http_request_duration_ms")
latencyHistogram.Record(ctx, 150, attribute.String("method", "POST"))

// Create gauge (via observable)
_, _ = meter.Int64ObservableGauge("active_connections",
    metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
        o.Observe(getActiveConnections())
        return nil
    }),
)
```

#### Option B: Use otel.Meter() Directly

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

// Get meter with custom scope name
meter := otel.Meter("my-service")

// Create counter
requestCounter, _ := meter.Int64Counter("http_requests_total")
requestCounter.Add(ctx, 1, metric.WithAttributes(
    attribute.String("method", "GET"),
    attribute.String("path", "/api/users"),
))
```

You can also use the metrics package directly:

```go
import "github.com/domesama/doakes/metrics"

// Get default meter (same as doakeswire.GetMeter())
meter := metrics.GetDefaultMeter()
```

### 3. Access Available Endpoints

The internal server exposes:

- `GET /` - Service information (JSON)
- `GET /_hc` - Health check endpoint
- `GET /metrics` - Prometheus metrics
- `GET /debug/pprof/` - CPU profiling, memory profiling, goroutine dumps, etc.

### 4. Check Server State

```go
if srv.IsRunning() {
    log.Println("Server is running")
}

if srv.IsHealthCheckEnabled() {
    log.Println("Health checks are enabled")
}

// Get the actual port when using :0 for dynamic port assignment
port := srv.GetRunningPort()
log.Printf("Server is running on port %d", port)

// Or get the full address
addr := srv.GetRunningAddress()
log.Printf("Server is running at %s", addr)
```

**Using Dynamic Port Assignment:**

When you want the OS to assign an available port, set the listen address to `:0`:

```go
os.Setenv("INTERNAL_SERVER_LISTEN_ADDR", ":0")

srv, cleanup, err := doakeswire.InitializeTelemetryServerWithAutoStart()
if err != nil {
    panic(err)
}
defer cleanup()

// Wait a bit for server to start
time.Sleep(100 * time.Millisecond)

// Get the actual port assigned by the OS
port := srv.GetRunningPort()
log.Printf("Telemetry server started on port %d", port)
log.Printf("Metrics available at http://localhost:%d/metrics", port)
```
```

### 5. Graceful Shutdown

```go
// Manual setup
defer srv.Stop()

// Wire auto-start
defer cleanup() // Calls Stop() for you
```

## Configuration

All configuration is done via environment variables:

### Required Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `OTEL_SERVICE_NAME` | Service name for metrics and tracing | `my-service` |
| `OTEL_SERVICE_VERSION` | Service version | `1.0.0` |

### Optional Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `INTERNAL_SERVER_LISTEN_ADDR` | `:28080` | Address for internal server to listen on |
| `INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION` | `1m` | Timeout for EnableHealthCheck() call |
| `INTERNAL_SERVER_HEALTH_CHECK_POLL_INTERVAL` | `15s` | How often to check if health checks are enabled |
| `PROMETHEUS_METRICS_NAME_VALIDATION` | _(none)_ | Set to `legacy` for relaxed metric name validation |
| `REGISTER_DEFAULT_PROMETHEUS_REGISTRY` | `false` | Register with default Prometheus registry |

### Histogram Boundaries

The library provides sensible defaults for histogram buckets:

**Default (millisecond metrics)**:
```
1, 5, 30, 50, 100, 200, 300, 500, 700, 1000,
1500, 2000, 2500, 3000, 5000, 7000, 9000, 10000
```

**Nanosecond metrics** (metrics ending in `_ns`):
```
1ns, 10ns, 100ns, 1μs, 10μs, 100μs, 1ms, 5ms,
30ms, 50ms, 100ms, 200ms, 300ms, 500ms, 700ms,
1s, 1.5s, 2s, 2.5s, 3s, 5s, 7s, 9s, 10s
```

### Example Configuration

```bash
export OTEL_SERVICE_NAME="user-service"
export OTEL_SERVICE_VERSION="2.3.1"
export INTERNAL_SERVER_LISTEN_ADDR=":8080"
export INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION="2m"
```

## Runtime Metrics

The following Go runtime metrics are automatically collected:

- `go_memory_used_bytes` - Current memory usage
- `go_memory_allocated_bytes_total` - Total bytes allocated
- `go_memory_allocations_total` - Total number of allocations
- `go_memory_gc_goal_bytes` - Heap size target for next GC
- `go_goroutine_count` - Number of goroutines
- `go_processor_limit` - CPU limit (GOMAXPROCS)
- `go_config_gogc_percent` - GC percentage target

## Examples

See the `/example` directory for complete examples:

- `example/basic/` - Manual setup without Wire
- `example/autostart/` - Using Wire with auto-start
- `example/wire/` - Custom Wire integration

## Testing

Run tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## Best Practices

1. **Always call EnableHealthCheck()** - Do it after initialization is complete
2. **Set realistic timeouts** - Give your service enough time to initialize
3. **Register meaningful health checks** - Check actual dependencies (DB, cache, APIs)
4. **Use appropriate metric types**:
   - **Counter**: Always-increasing values (requests, errors)
   - **Histogram**: Distributions (latency, size)
   - **Gauge**: Current state (active connections, queue length)
5. **Use attributes for cardinality** - Add labels to metrics for filtering
6. **Don't forget cleanup** - Use `defer cleanup()` or `defer srv.Stop()`

## License

This project is licensed under the MIT License.

---

