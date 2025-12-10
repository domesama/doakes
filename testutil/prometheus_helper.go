package testutil

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	prometheusClient "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

// PrometheusHelper helps test Prometheus metrics endpoints.
type PrometheusHelper struct {
	httpClient http.Client
	port       int
	parser     expfmt.TextParser
}

// NewPrometheusHelper creates a helper for testing Prometheus metrics.
func NewPrometheusHelper(port int) *PrometheusHelper {
	return &PrometheusHelper{
		httpClient: http.Client{},
		port:       port,
		parser:     expfmt.NewTextParser(model.UTF8Validation),
	}
}

// ParseMetrics fetches and parses metrics from the /metrics endpoint.
func (h *PrometheusHelper) ParseMetrics(t *testing.T) *Metrics {
	ctx := context.Background()
	url := fmt.Sprintf("http://localhost:%d/metrics", h.port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	assert.NoError(t, err)

	resp, err := h.httpClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	defer func() {
		_ = resp.Body.Close()
	}()

	metricFamilies, err := h.parser.TextToMetricFamilies(resp.Body)
	assert.NoError(t, err)

	return &Metrics{
		families: metricFamilies,
	}
}

// Metrics represents parsed Prometheus metrics.
type Metrics struct {
	families map[string]*prometheusClient.MetricFamily
}

// Get returns all metrics matching the name and labels.
func (m *Metrics) Get(name string, labels map[string]string) []*prometheusClient.Metric {
	family, ok := m.families[name]
	if !ok {
		return nil
	}

	metrics := family.GetMetric()
	if len(labels) == 0 {
		return metrics
	}

	var matched []*prometheusClient.Metric
	for _, metric := range metrics {
		if hasAllLabels(metric, labels) {
			matched = append(matched, metric)
		}
	}

	return matched
}

// GetSingle returns a single metric matching the name and labels.
func (m *Metrics) GetSingle(t *testing.T, name string, labels map[string]string) *prometheusClient.Metric {
	metrics := m.Get(name, labels)

	if len(metrics) == 0 {
		return nil
	}
	if len(metrics) > 1 {
		assert.Fail(t, "multiple metrics found", "expected only one metric %s %v", name, labels)
		return nil
	}

	return metrics[0]
}

// AssertNoMetric asserts that no metric exists with the given name and labels.
func (m *Metrics) AssertNoMetric(t *testing.T, name string, labels map[string]string) {
	metrics := m.Get(name, labels)
	assert.Empty(t, metrics, "expected no metric %s %v", name, labels)
}

// AssertCounter asserts a counter metric has the expected value.
func (m *Metrics) AssertCounter(t *testing.T, name string, labels map[string]string, expected float64) {
	currentMetric := m.GetSingle(t, name, labels)
	if !assert.NotNil(t, currentMetric, "currentMetric %s %v not found", name, labels) {
		return
	}

	actual := currentMetric.Counter.GetValue()
	assert.InDelta(t, expected, actual, 0.0000001, "counter %s %v", name, labels)
}

// AssertHistogramCount asserts a histogram's sample count.
func (m *Metrics) AssertHistogramCount(t *testing.T, name string, labels map[string]string, expected uint64) {
	metric := m.GetSingle(t, name, labels)
	if !assert.NotNil(t, metric, "metric %s %v not found", name, labels) {
		return
	}

	actual := metric.Histogram.GetSampleCount()
	assert.Equal(t, expected, actual, "histogram count %s %v", name, labels)
}

// AssertMetricExists asserts that a metric exists with the expected type.
func (m *Metrics) AssertMetricExists(t *testing.T, name string, labels map[string]string,
	expectedType prometheusClient.MetricType) {
	family, ok := m.families[name]
	if !assert.True(t, ok, "metric family %s not found", name) {
		return
	}

	metrics := m.Get(name, labels)
	assert.NotEmpty(t, metrics, "metric %s %v not found", name, labels)

	assert.Equal(t, expectedType, family.GetType(), "metric type for %s", name)
}

// AssertCounterIncrease asserts that a counter increased by the expected amount.
func AssertCounterIncrease(t *testing.T, before, after *Metrics, name string, labels map[string]string,
	expectedIncrease float64) {
	beforeMetric := before.GetSingle(t, name, labels)

	expectedValue := expectedIncrease
	if beforeMetric != nil {
		expectedValue += beforeMetric.Counter.GetValue()
	}

	if expectedValue == 0 {
		after.AssertNoMetric(t, name, labels)
	} else {
		after.AssertCounter(t, name, labels, expectedValue)
	}
}

// AssertHistogramIncrease asserts that a histogram count increased by the expected amount.
func AssertHistogramIncrease(t *testing.T, before, after *Metrics, name string, labels map[string]string,
	expectedIncrease uint64) {
	beforeMetric := before.GetSingle(t, name, labels)

	expectedCount := expectedIncrease
	if beforeMetric != nil {
		expectedCount += beforeMetric.Histogram.GetSampleCount()
	}

	if expectedCount == 0 {
		after.AssertNoMetric(t, name, labels)
	} else {
		after.AssertHistogramCount(t, name, labels, expectedCount)
	}
}

func hasAllLabels(metric *prometheusClient.Metric, selectedLabels map[string]string) bool {
	labelsByName := make(map[string]string)

	for _, label := range metric.Label {
		labelsByName[label.GetName()] = label.GetValue()
	}

	for key, expectedValue := range selectedLabels {
		actualValue, ok := labelsByName[key]
		if !ok || actualValue != expectedValue {
			return false
		}
	}

	return true
}
