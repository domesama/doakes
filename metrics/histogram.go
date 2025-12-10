package metrics

import (
	"github.com/domesama/doakes/config"
	"go.opentelemetry.io/otel/sdk/metric"
)

// CreateHistogramViews creates OpenTelemetry metric views for histogram configuration.
// Named patterns (e.g., "*_ns") get their specific boundaries, all others use defaults.
func CreateHistogramViews(metricsConfig config.MetricsConfig) []metric.View {
	var views []metric.View

	namedHistogramViews := createNamedHistogramViews(metricsConfig.HistogramBoundariesByName)
	views = append(views, namedHistogramViews...)

	defaultHistogramView := createDefaultHistogramView(metricsConfig.DefaultHistogramBoundaries)
	views = append(views, defaultHistogramView)

	return views
}

func createNamedHistogramViews(boundariesByName map[string][]float64) []metric.View {
	var views []metric.View

	for metricNamePattern, boundaries := range boundariesByName {
		view := metric.NewView(
			metric.Instrument{
				Name: metricNamePattern,
				Kind: metric.InstrumentKindHistogram,
			},
			metric.Stream{
				Aggregation: metric.AggregationExplicitBucketHistogram{
					Boundaries: boundaries,
				},
			},
		)
		views = append(views, view)
	}

	return views
}

func createDefaultHistogramView(boundaries []float64) metric.View {
	return metric.NewView(
		metric.Instrument{
			Kind: metric.InstrumentKindHistogram,
		},
		metric.Stream{
			Aggregation: metric.AggregationExplicitBucketHistogram{
				Boundaries: boundaries,
			},
		},
	)
}
