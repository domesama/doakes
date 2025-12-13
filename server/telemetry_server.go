package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/domesama/doakes/config"
	"github.com/domesama/doakes/healthcheck"
	"github.com/domesama/doakes/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	internalhttp "github.com/domesama/doakes/http"
)

// TelemetryServer manages the internal observability server that exposes metrics,
// health checks, and profiling endpoints.
type TelemetryServer struct {
	config          config.TelemetryServerConfig
	httpServer      *internalhttp.Server
	healthCheck     *healthcheck.Handler
	metricsProvider *metrics.Provider

	mutex   sync.RWMutex
	running bool
	// healthCheckWaiter monitors if EnableHealthCheck() is called within timeout
	// to prevent services from passing health checks before they're ready
	healthCheckWaiter *healthCheckWaiter
}

// Options contains configuration for creating a new TelemetryServer.
type Options struct {
	Resource              *resource.Resource
	MetricsConfig         config.MetricsConfig
	TelemetryServerConfig config.TelemetryServerConfig
	ServiceName           string
	ServiceVersion        string
}

// New creates a new TelemetryServer with the provided options.
// It initializes the metrics provider, HTTP server, and health check handler.
func New(opts Options) (*TelemetryServer, error) {
	if opts.Resource == nil {
		opts.Resource = resource.Default()
	}

	serviceName := ExtracResourceByKey(semconv.ServiceNameKey, opts.Resource)
	serviceVersion := ExtracResourceByKey(semconv.ServiceVersionKey, opts.Resource)

	healthCheckHandler := internalhttp.NewHealthCheckHandler(serviceName)

	metricsProvider, err := metrics.NewProvider(opts.Resource, opts.MetricsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics provider: %w", err)
	}

	indexHandler := internalhttp.CreateIndexHandler(serviceName, serviceVersion)

	router := internalhttp.NewRouter(
		internalhttp.RouterConfig{
			HealthCheckHandler: healthCheckHandler,
			MetricsHandler:     metricsProvider.HTTPHandler(),
			IndexHandler:       indexHandler,
		},
	)

	httpServer := internalhttp.NewServer(router)

	server := &TelemetryServer{
		config:          opts.TelemetryServerConfig,
		httpServer:      httpServer,
		healthCheck:     healthCheckHandler,
		metricsProvider: metricsProvider,
	}

	return server, nil
}

// RegisterHealthCheck adds a health check with the given name.
// The check function will be called when the health check endpoint is hit.
func (s *TelemetryServer) RegisterHealthCheck(name string, checkFn healthcheck.CheckFunction) {
	s.healthCheck.RegisterCheck(name, checkFn)
}

// EnableHealthCheck activates the health check endpoint.
// This must be called after registration or the endpoint will return 503.
// This is intentional to prevent premature health check passes during startup.
func (s *TelemetryServer) EnableHealthCheck() {
	s.healthCheck.Enable()
}

// IsHealthCheckEnabled returns true if health checks are enabled.
func (s *TelemetryServer) IsHealthCheckEnabled() bool {
	return s.healthCheck.IsEnabled()
}

// Start begins serving HTTP requests on the configured address.
func (s *TelemetryServer) Start() error {
	return s.StartWithAddress(s.config.ListenAddress)
}

// StartWithAddress begins serving HTTP requests on the specified address.
// The health check watcher will start monitoring for EnableHealthCheck() calls.
func (s *TelemetryServer) StartWithAddress(address string) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		slog.Info("TelemetryServer already running", "address", address)
		return nil
	}
	s.running = true
	s.mutex.Unlock()

	slog.Info("Starting internal telemetry server", "address", address)

	s.startHealthCheckWatcher()

	go func() {
		err := s.httpServer.Start(address)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("TelemetryServer failed", "error", err)
			panic(err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server.
// It stops the HTTP server, metrics provider, and health check watcher.
func (s *TelemetryServer) Stop() error {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return nil
	}
	s.running = false
	s.mutex.Unlock()

	s.stopHealthCheckWatcher()

	slog.Info("Shutting down internal telemetry server")

	if err := s.httpServer.Shutdown(); err != nil {
		return err
	}

	s.metricsProvider.Cleanup()

	slog.Info("internal telemetry server stopped")
	return nil
}

// IsRunning returns true if the server is currently running.
func (s *TelemetryServer) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// GetRunningAddress returns the actual address the server is listening on.
// This is useful when using ":0" to get the OS-assigned port.
// Returns empty string if the server hasn't started yet.
func (s *TelemetryServer) GetRunningAddress() string {
	return s.httpServer.ActualAddress()
}

// GetRunningPort returns the actual port the server is listening on.
// This is useful when using ":0" to get the OS-assigned port.
// Returns 0 if the server hasn't started yet or if port cannot be determined.
func (s *TelemetryServer) GetRunningPort() int {
	addr := s.httpServer.ActualAddress()
	if addr == "" {
		return 0
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}

	return portNum
}

func (s *TelemetryServer) startHealthCheckWatcher() {
	s.healthCheckWaiter = newHealthCheckWaiter(
		s,
		s.config.HealthCheckEnableTimeout,
		s.config.HealthCheckPollInterval,
	)
	s.healthCheckWaiter.start()
}

func (s *TelemetryServer) stopHealthCheckWatcher() {
	if s.healthCheckWaiter != nil {
		s.healthCheckWaiter.stop()
	}
}

func ExtracResourceByKey(key attribute.Key, resource *resource.Resource) (result string) {
	result = fmt.Sprintf("unknown-%s", key)
	if resource == nil {
		return
	}

	resourceValue, ok := resource.Set().Value(key)
	if !ok {
		return
	}

	return resourceValue.AsString()
}
