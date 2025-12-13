package metrics

import (
	"github.com/domesama/doakes/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// CreateHistogramViews creates OpenTelemetry metric views for histogram configuration.
// Named patterns (e.g., "*_ns") get their specific boundaries, all others use defaults.
func CreateHistogramViews(metricsConfig config.MetricsConfig) []sdkmetric.View {
	var views []sdkmetric.View

	namedHistogramViews := createNamedHistogramViews(metricsConfig.HistogramBoundariesByName)
	views = append(views, namedHistogramViews...)

	defaultHistogramView := createDefaultHistogramView(metricsConfig.DefaultHistogramBoundaries)
	views = append(views, defaultHistogramView)

	return views
}

func createNamedHistogramViews(boundariesByName map[string][]float64) []sdkmetric.View {
	var views []sdkmetric.View

	for metricNamePattern, boundaries := range boundariesByName {
		view := sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: metricNamePattern,
				Kind: sdkmetric.InstrumentKindHistogram,
			},
			sdkmetric.Stream{
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: boundaries,
				},
			},
		)
		views = append(views, view)
	}

	return views
}

func createDefaultHistogramView(boundaries []float64) sdkmetric.View {
	return sdkmetric.NewView(
		sdkmetric.Instrument{
			Kind: sdkmetric.InstrumentKindHistogram,
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: boundaries,
			},
		},
	)
}
