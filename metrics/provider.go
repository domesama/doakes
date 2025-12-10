// Package metrics provides OpenTelemetry metrics with Prometheus exporter.
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/domesama/doakes/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Provider manages the OpenTelemetry meter provider and Prometheus exporter.
type Provider struct {
	registry      *prometheus.Registry
	exporter      *otelprom.Exporter
	meterProvider *metric.MeterProvider
	httpHandler   http.Handler
	cleanupFuncs  []func()
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

	provider := &Provider{
		registry:      registry,
		exporter:      exporter,
		meterProvider: meterProvider,
		httpHandler:   httpHandler,
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
	views []metric.View) *metric.MeterProvider {
	// Add default view for all metrics
	defaultView := metric.NewView(
		metric.Instrument{Name: "*"},
		metric.Stream{Aggregation: metric.AggregationDefault{}},
	)
	views = append(views, defaultView)

	return metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithView(views...),
		metric.WithResource(res),
	)
}

func initializeRuntimeMetrics(meterProvider *metric.MeterProvider) error {

	return runtime.Start(runtime.WithMeterProvider(meterProvider))
}

func setGlobalMeterProvider(meterProvider *metric.MeterProvider) {
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
