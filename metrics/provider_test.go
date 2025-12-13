package metrics

import (
	"context"
	"os"
	"testing"

	"github.com/domesama/doakes/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

func TestGetDefaultMeter(t *testing.T) {
	// Set service name
	os.Setenv("OTEL_SERVICE_NAME", "test-service")
	defer os.Unsetenv("OTEL_SERVICE_NAME")

	// Create resource
	res, err := resource.New(
		nil,
		resource.WithAttributes(semconv.ServiceNameKey.String("test-service")),
	)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	// Create provider (this sets global meter provider)
	provider, err := NewProvider(res, config.DefaultMetricsConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Cleanup()

	// Get default meter
	meter := GetDefaultMeter()
	if meter == nil {
		t.Fatal("GetDefaultMeter() returned nil")
	}

	// Create a counter to verify it works
	counter, err := meter.Int64Counter("test_counter")
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Record some values
	counter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("test", "value")))
}

func TestProviderGetMeter(t *testing.T) {
	// Create resource with service name
	res, err := resource.New(
		nil,
		resource.WithAttributes(semconv.ServiceNameKey.String("my-test-service")),
	)
	if err != nil {
		t.Fatalf("failed to create resource: %v", err)
	}

	// Create provider
	provider, err := NewProvider(res, config.DefaultMetricsConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Cleanup()

	// Get meter from provider
	meter := provider.GetMeter()
	if meter == nil {
		t.Fatal("provider.GetMeter() returned nil")
	}

	// Create a histogram to verify it works
	histogram, err := meter.Int64Histogram("test_histogram")
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}

	// Record some values
	histogram.Record(context.Background(), 100, metric.WithAttributes(attribute.String("test", "value")))
}

func TestGetServiceNameFromEnv(t *testing.T) {
	tests := []struct {
		name            string
		otelServiceName string
		expected        string
	}{
		{
			name:            "OTEL_SERVICE_NAME is set",
			otelServiceName: "my-otel-service",
			expected:        "my-otel-service",
		},
		{
			name:            "Default to unknown-service",
			otelServiceName: "",
			expected:        "unknown-service",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Clean environment
				os.Unsetenv("OTEL_SERVICE_NAME")

				// Set test values
				if tt.otelServiceName != "" {
					os.Setenv("OTEL_SERVICE_NAME", tt.otelServiceName)
				}

				// Test
				result := getServiceNameFromEnv()
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}

				// Cleanup
				os.Unsetenv("OTEL_SERVICE_NAME")
			},
		)
	}
}
