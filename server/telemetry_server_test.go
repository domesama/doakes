package server_test

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"testing"
	"time"

	"github.com/domesama/doakes/doakeswire"
	"github.com/domesama/doakes/testutil"
	prometheusClient "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	otelRuntimeMetrics = map[string]prometheusClient.MetricType{
		"go_memory_used_bytes":            prometheusClient.MetricType_GAUGE,
		"go_memory_allocated_bytes_total": prometheusClient.MetricType_COUNTER,
		"go_memory_allocations_total":     prometheusClient.MetricType_COUNTER,
		"go_memory_gc_goal_bytes":         prometheusClient.MetricType_GAUGE,
		"go_goroutine_count":              prometheusClient.MetricType_GAUGE,
		"go_processor_limit":              prometheusClient.MetricType_GAUGE,
		"go_config_gogc_percent":          prometheusClient.MetricType_GAUGE,
	}
)

func TestInternalServer(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
	_ = os.Setenv("INTERNAL_SERVER_LISTEN_ADDR", ":28080")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	// Register health checks
	srv.RegisterHealthCheck(
		"test", func() error {
			return nil
		},
	)
	srv.EnableHealthCheck()

	// Get meters from global provider
	meter := otel.Meter("test")
	counter1, err := meter.Int64Counter("test_counter")
	assert.NoError(t, err)
	counter2, err := meter.Int64Counter("test_counter")
	assert.NoError(t, err)
	histogram1, err := meter.Int64Histogram("test_histogram")
	assert.NoError(t, err)
	histogram2, err := meter.Int64Histogram("test_histogram")
	assert.NoError(t, err)

	attrs := metric.WithAttributes(
		attribute.String("attr1", "val1"),
		attribute.String("attr2", "val2"),
	)

	// Wait for server to be ready
	assert.True(t, srv.IsRunning())

	helper := testutil.NewPrometheusHelper(28080)

	// Verify metrics don't exist initially
	m1 := helper.ParseMetrics(t)
	m1.AssertNoMetric(t, "test_counter_total", map[string]string{"attr1": "val1", "attr2": "val2"})
	m1.AssertNoMetric(t, "test_histogram", map[string]string{"attr1": "val1", "attr2": "val2"})

	ctx := context.Background()
	c := http.Client{}

	// Wait for health check to be ready and record metrics
	for i := 0; i < 10; i++ {
		counter1.Add(ctx, 1, attrs)
		counter2.Add(ctx, 10, attrs)
		counter2.Add(ctx, 1)

		histogram1.Record(ctx, 1, attrs)
		histogram2.Record(ctx, 10, attrs)
		histogram2.Record(ctx, 1)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:28080/_hc", nil)
		assert.NoError(t, err)
		resp, err := c.Do(req)
		if err == nil {
			if resp != nil && resp.Body != nil {
				assert.NoError(t, resp.Body.Close())
			}
			if resp.StatusCode == 200 {
				// Verify metrics without certain labels don't exist
				helper.ParseMetrics(t).AssertNoMetric(t, "test_counter_total", map[string]string{"attr1": "val"})
				helper.ParseMetrics(t).AssertNoMetric(t, "test_histogram", map[string]string{"attr1": "val"})

				// Verify metric increases
				m2 := helper.ParseMetrics(t)
				testutil.AssertCounterIncrease(
					t, m1, m2, "test_counter_total",
					map[string]string{"attr1": "val1", "attr2": "val2"}, 11,
				)
				testutil.AssertHistogramIncrease(
					t, m1, m2, "test_histogram",
					map[string]string{"attr1": "val1", "attr2": "val2"}, 2,
				)

				return
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
	assert.Fail(t, "too long waiting for server to start")
}

func createGarbage() {
	var sink [][]byte
	for i := 0; i < 20_000; i++ {
		b := make([]byte, 64*1024) // 64KB
		sink = append(sink, b)
		if i%2 == 0 { // drop half to become garbage
			sink[i] = nil
		}
	}
	_ = sink
}

func TestInternalServerDefaultMetric(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
	_ = os.Setenv("INTERNAL_SERVER_LISTEN_ADDR", ":28080")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	srv.RegisterHealthCheck(
		"test", func() error {
			return nil
		},
	)
	srv.EnableHealthCheck()

	assert.True(t, srv.IsRunning())

	helper := testutil.NewPrometheusHelper(28080)

	old := debug.SetGCPercent(5)
	t.Cleanup(
		func() {
			debug.SetGCPercent(old)
		},
	)

	// Simulate gc to generate runtime metrics
	createGarbage()
	runtime.GC()

	// Wait a bit for metrics to be collected
	time.Sleep(500 * time.Millisecond)

	m := helper.ParseMetrics(t)
	assertMetricsExist(t, otelRuntimeMetrics, m)
}

func assertMetricsExist(t *testing.T, expectedMetrics map[string]prometheusClient.MetricType, m *testutil.Metrics) {
	for name, expectedType := range expectedMetrics {
		m.AssertMetricExists(t, name, nil, expectedType)
	}
}

func TestServerCreation(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	assert.True(t, srv.IsRunning())
	assert.False(t, srv.IsHealthCheckEnabled())
}

func TestServerHealthCheck(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	checkCalled := false
	srv.RegisterHealthCheck(
		"test", func() error {
			checkCalled = true
			return nil
		},
	)

	// Health check not enabled yet
	assert.False(t, srv.IsHealthCheckEnabled())

	// Enable health check
	srv.EnableHealthCheck()
	assert.True(t, srv.IsHealthCheckEnabled())

	// Wait a bit for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Make health check request
	resp, err := http.Get("http://localhost:28080/_hc")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
		_ = resp.Body.Close()
	}

	assert.True(t, checkCalled, "health check function should have been called")
}

func TestServerMetricsEndpoint(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	srv.EnableHealthCheck()

	// Wait for server
	time.Sleep(200 * time.Millisecond)

	// Test metrics endpoint
	resp, err := http.Get("http://localhost:28080/metrics")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
		_ = resp.Body.Close()
	}
}

func TestServerIndexEndpoint(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("OTEL_SERVICE_VERSION", "1.2.3")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	srv.EnableHealthCheck()

	// Wait for server
	time.Sleep(200 * time.Millisecond)

	// Test index endpoint
	resp, err := http.Get("http://localhost:28080/")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
		_ = resp.Body.Close()
	}
}

func TestServerHealthCheckTimeout(t *testing.T) {
	// This test verifies that calling EnableHealthCheck() prevents the timeout panic.
	// We test the positive case (enabling works) rather than testing the panic itself,
	// since panics in goroutines are hard to test cleanly.

	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "1s")
	_ = os.Setenv("INTERNAL_SERVER_HEALTH_CHECK_POLL_INTERVAL", "100ms")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	defer cleanUpFn()

	// Wait a bit to ensure watcher has started
	time.Sleep(200 * time.Millisecond)

	// Enable health check BEFORE timeout
	srv.EnableHealthCheck()

	// Wait past the timeout period
	time.Sleep(1500 * time.Millisecond)

	// If we reach here without panic, the test passes
	// This verifies that EnableHealthCheck() successfully stops the watcher
	assert.True(t, srv.IsHealthCheckEnabled())
}

func TestServerGetRunningPort(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_LISTEN_ADDR", ":0") // Use port 0 to get OS-assigned port
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, cleanUpFn, err := doakeswire.InitializeTelemetryServerWithAutoStart()
	assert.NoError(t, err)
	assert.NotNil(t, srv)
	t.Cleanup(cleanUpFn)

	srv.EnableHealthCheck()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Get the actual running port
	port := srv.GetRunningPort()
	assert.NotEqual(t, 0, port, "Running port should not be 0")
	assert.Greater(t, port, 1024, "Port should be greater than 1024 (ephemeral port range)")

	// Get the actual running address
	addr := srv.GetRunningAddress()
	assert.NotEmpty(t, addr, "Running address should not be empty")
	assert.Contains(t, addr, ":", "Address should contain port separator")

	// Verify we can actually connect to the port
	resp, err := http.Get("http://localhost:" + strconv.Itoa(port) + "/_hc")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.Equal(t, 200, resp.StatusCode)
		_ = resp.Body.Close()
	}
}

func TestServerGetRunningPortBeforeStart(t *testing.T) {
	_ = os.Setenv("OTEL_SERVICE_NAME", "test-service")
	_ = os.Setenv("INTERNAL_SERVER_WAIT_ENABLE_HEALTH_CHECK_DURATION", "5s")

	srv, err := doakeswire.InitializeTelemetryServer()
	assert.NoError(t, err)
	assert.NotNil(t, srv)

	// Server not started yet, should return 0
	port := srv.GetRunningPort()
	assert.Equal(t, 0, port, "Port should be 0 before server starts")

	addr := srv.GetRunningAddress()
	assert.Empty(t, addr, "Address should be empty before server starts")
}
