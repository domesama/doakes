// Package wire provides Google Wire dependency injection providers for internal telemetry.
package doakeswire

import (
	"os"

	"github.com/domesama/doakes/config"
	"github.com/domesama/doakes/server"
	"github.com/google/wire"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// TelemetrySet contains all the default Wire providers for the internal telemetry server.
var TelemetrySet = wire.NewSet(
	ProvideResource,
	ProvideMetricsConfig,
	ProvideServerOptions,
	ProvideTelemetryServerConfig,

	server.New,
)

// TelemetrySetWithAutoStart creates a server that starts automatically.
// Returns (*server.TelemetryServer, cleanup func(), error).
var TelemetrySetWithAutoStart = wire.NewSet(
	ProvideResource,
	ProvideMetricsConfig,
	ProvideServerOptions,
	ProvideTelemetryServerConfig,

	ProvideServer,
)

// ProvideTelemetryServerConfig loads server configuration from environment variables.
func ProvideTelemetryServerConfig() (config.TelemetryServerConfig, error) {
	return config.LoadServerConfig()
}

// ProvideMetricsConfig returns the default metrics configuration.
func ProvideMetricsConfig() config.MetricsConfig {
	return config.DefaultMetricsConfig()
}

// ProvideResource creates an OpenTelemetry resource from environment variables.
// Reads OTEL_SERVICE_NAME (or legacy APP_NAME) and OTEL_SERVICE_VERSION.
func ProvideResource() (*resource.Resource, error) {
	attributes := make([]attribute.KeyValue, 0)

	// Service name - prefer OTEL_SERVICE_NAME over legacy APP_NAME
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = os.Getenv("APP_NAME")
	}
	if serviceName != "" {
		attributes = append(attributes, semconv.ServiceNameKey.String(serviceName))
	}

	// Service version
	serviceVersion := os.Getenv("OTEL_SERVICE_VERSION")
	if serviceVersion != "" {
		attributes = append(attributes, semconv.ServiceVersionKey.String(serviceVersion))
	}

	return resource.New(
		nil,
		resource.WithAttributes(attributes...),
	)
}

// ProvideServerOptions creates server options from the provided dependencies.
func ProvideServerOptions(
	res *resource.Resource,
	metricsConfig config.MetricsConfig,
	serverConfig config.TelemetryServerConfig,
) server.Options {
	return server.Options{
		Resource:              res,
		MetricsConfig:         metricsConfig,
		TelemetryServerConfig: serverConfig,
		// ServiceName:   serviceName, TODO: Read service name from env
	}
}

// ProvideServer creates and starts an internal server, returning it with a cleanup function.
// This is similar to Provideinternal telemetryV2 but for the simplified V2 architecture.
//
// The server is started but health checks are NOT enabled.
// Call srv.EnableHealthCheck() after your initialization is complete.
//
// Usage:
//
//	srv, cleanup, err := wire.ProvideServer()
//	if err != nil {
//	    return err
//	}
//	defer cleanup()
//
//	// Register your health checks
//	srv.RegisterHealthCheck("database", checkDB)
//
//	// Enable after initialization
//	srv.EnableHealthCheck()
func ProvideServer(opts server.Options) (*server.TelemetryServer, func(), error) {
	srv, err := server.New(opts)
	if err != nil {
		return nil, func() {}, err
	}

	if err := srv.Start(); err != nil {
		return nil, func() {}, err
	}

	cleanup := func() {
		_ = srv.Stop()
	}

	return srv, cleanup, nil
}
