// Package metrics provides OpenTelemetry metrics with Prometheus exporter.
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/domesama/doakes/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// Provider manages the OpenTelemetry meter provider and Prometheus exporter.
type Provider struct {
	registry      *prometheus.Registry
	exporter      *otelprom.Exporter
	meterProvider *sdkmetric.MeterProvider
	httpHandler   http.Handler
	cleanupFuncs  []func()
	serviceName   string
}

// NewProvider creates a new metrics provider with Prometheus export.
// It configures histogram views, starts runtime metrics, and sets the global meter provider.
func NewProvider(res *resource.Resource, metricsConfig config.MetricsConfig) (*Provider, error) {
	registry := createPrometheusRegistry(metricsConfig)

	exporter, err := createOtelPrometheusExporter(registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	histogramViews := CreateHistogramViews(metricsConfig)
	meterProvider := createMeterProvider(res, exporter, histogramViews)

	if err := initializeRuntimeMetrics(meterProvider); err != nil {
		return nil, fmt.Errorf("failed to initialize runtime metrics: %w", err)
	}

	setGlobalMeterProvider(meterProvider)

	httpHandler := createPrometheusHTTPHandler(registry)

	// Extract service name from resource
	serviceName := extractServiceName(res)

	provider := &Provider{
		registry:      registry,
		exporter:      exporter,
		meterProvider: meterProvider,
		httpHandler:   httpHandler,
		serviceName:   serviceName,
		cleanupFuncs: []func(){
			func() { _ = exporter.Shutdown(context.Background()) },
			func() { _ = meterProvider.Shutdown(context.Background()) },
		},
	}

	return provider, nil
}

// HTTPHandler returns the HTTP handler for the Prometheus metrics endpoint.
func (p *Provider) HTTPHandler() http.Handler {
	return p.httpHandler
}

// Cleanup shuts down the exporter and meter provider.
func (p *Provider) Cleanup() {
	for _, cleanup := range p.cleanupFuncs {
		cleanup()
	}
}

func createPrometheusRegistry(metricsConfig config.MetricsConfig) *prometheus.Registry {
	// Use NewPedanticRegistry to have more control over validation
	// This avoids the "unset" validation scheme error
	registry := prometheus.NewRegistry()

	if metricsConfig.RegisterDefaultPrometheusRegistry {
		prometheus.DefaultRegisterer = registry
	}

	return registry
}

func createOtelPrometheusExporter(registry *prometheus.Registry) (*otelprom.Exporter, error) {
	return otelprom.New(otelprom.WithRegisterer(registry))
}

func createMeterProvider(res *resource.Resource, exporter *otelprom.Exporter,
	views []sdkmetric.View) *sdkmetric.MeterProvider {
	// Add default view for all metrics
	defaultView := sdkmetric.NewView(
		sdkmetric.Instrument{Name: "*"},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationDefault{}},
	)
	views = append(views, defaultView)

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(views...),
		sdkmetric.WithResource(res),
	)
}

func initializeRuntimeMetrics(meterProvider *sdkmetric.MeterProvider) error {

	return runtime.Start(runtime.WithMeterProvider(meterProvider))
}

func setGlobalMeterProvider(meterProvider *sdkmetric.MeterProvider) {
	otel.SetMeterProvider(meterProvider)
}

func createPrometheusHTTPHandler(registry *prometheus.Registry) http.Handler {
	logger := &promLogger{}

	return promhttp.HandlerFor(
		registry, promhttp.HandlerOpts{
			ErrorLog: logger,
		},
	)
}

type promLogger struct{}

func (l *promLogger) Println(values ...interface{}) {
	if len(values) == 0 {
		return
	}

	format, ok := values[0].(string)
	if !ok {
		slog.Info("prometheus", "values", values)
		return
	}

	slog.Info(fmt.Sprintf(format, values[1:]...), "module", "prometheus")
}

// GetMeter returns a Meter scoped to the service name from the provider.
// This is a convenience method for getting a meter without manually specifying the scope.
func (p *Provider) GetMeter() metric.Meter {
	return otel.GetMeterProvider().Meter(p.serviceName)
}

// GetDefaultMeter returns a Meter scoped to the OTEL_SERVICE_NAME environment variable.
// This is a package-level convenience function that can be called after the provider is initialized.
// It uses the global meter provider set by NewProvider.
func GetDefaultMeter() metric.Meter {
	serviceName := getServiceNameFromEnv()
	return otel.GetMeterProvider().Meter(serviceName)
}

// extractServiceName extracts the service name from the OpenTelemetry resource.
// Falls back to environment variable or "unknown-service" if not found.
func extractServiceName(res *resource.Resource) string {
	if res != nil {
		if value, ok := res.Set().Value(semconv.ServiceNameKey); ok {
			return value.AsString()
		}
	}
	return getServiceNameFromEnv()
}

// getServiceNameFromEnv reads the service name from OTEL_SERVICE_NAME environment variable.
func getServiceNameFromEnv() string {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "unknown-service"
	}
	return serviceName
}
