// Package config provides configuration for the internal telemetry server.
package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// TelemetryServerConfig contains HTTP server configuration.
type TelemetryServerConfig struct {
	ListenAddress            string        `envconfig:"INTERNAL_SERVER_LISTEN_ADDR" default:":28080"`
	HealthCheckEnableTimeout time.Duration `envconfig:"INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION" default:"1m"`
	HealthCheckPollInterval  time.Duration `envconfig:"INTERNAL_SERVER_HEALTH_CHECK_POLL_INTERVAL" default:"15s"`
}

// MetricsConfig contains OpenTelemetry metrics configuration.
type MetricsConfig struct {
	// DefaultHistogramBoundaries are used for all histograms not matching a specific pattern
	DefaultHistogramBoundaries []float64
	// HistogramBoundariesByName maps metric name patterns to custom boundaries (e.g., "*_ns" for nanosecond metrics)
	HistogramBoundariesByName         map[string][]float64
	RegisterDefaultPrometheusRegistry bool `envconfig:"REGISTER_DEFAULT_PROMETHEUS_REGISTRY" default:"false"`
}

// LoadServerConfig loads server configuration from environment variables.
func LoadServerConfig() (TelemetryServerConfig, error) {
	var config TelemetryServerConfig
	err := envconfig.Process("", &config)
	return config, err
}

// DefaultMetricsConfig returns a metrics configuration with sensible histogram boundaries.
// Millisecond metrics use 1-10000ms boundaries, nanosecond metrics use 1ns-10s boundaries.
func DefaultMetricsConfig() MetricsConfig {
	config := MetricsConfig{
		DefaultHistogramBoundaries: []float64{
			1, 5, 30, 50, 100, 200, 300, 500, 700, 1000,
			1500, 2000, 2500, 3000, 5000, 7000, 9000, 10000,
		},
		HistogramBoundariesByName: map[string][]float64{
			"*_ns": {
				1, 10, 100, 1000, 10000, 100000, 1000000, 5000000,
				30000000, 50000000, 100000000, 200000000, 300000000,
				500000000, 700000000, 1000000000, 1500000000, 2000000000,
				2500000000, 3000000000, 5000000000, 7000000000, 9000000000, 10000000000,
			},
		},
	}

	envconfig.MustProcess("", &config)
	return config
}
