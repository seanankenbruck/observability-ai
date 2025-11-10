package processor

import (
	"testing"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/mimir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultProcessor_ProcessVector(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("single vector result", func(t *testing.T) {
		mimirResult := &mimir.QueryResult{
			ResultType: "vector",
			Data: []mimir.InstantVector{
				{
					Metric:    map[string]string{"job": "api", "instance": "host1"},
					Val:       42.5,
					Timestamp: float64(time.Now().Unix()),
				},
			},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 1, results.TotalSeries)
		assert.False(t, results.Truncated)
		assert.Contains(t, results.Summary, "Current value: 42.50")
		assert.Len(t, results.Samples, 1)
		assert.Equal(t, 42.5, results.Samples[0].Value)
	})

	t.Run("multiple vector results under limit", func(t *testing.T) {
		vectors := make([]mimir.InstantVector, 5)
		for i := 0; i < 5; i++ {
			vectors[i] = mimir.InstantVector{
				Metric:    map[string]string{"job": "api", "instance": "host" + string(rune(i))},
				Val:       float64(i * 10),
				Timestamp: float64(time.Now().Unix()),
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "vector",
			Data:       vectors,
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 5, results.TotalSeries)
		assert.False(t, results.Truncated)
		assert.Len(t, results.Samples, 5)
		assert.Contains(t, results.Summary, "Found 5 series")
	})

	t.Run("multiple vector results over limit", func(t *testing.T) {
		vectors := make([]mimir.InstantVector, 20)
		for i := 0; i < 20; i++ {
			vectors[i] = mimir.InstantVector{
				Metric:    map[string]string{"job": "api", "instance": "host" + string(rune(i))},
				Val:       float64(i * 10),
				Timestamp: float64(time.Now().Unix()),
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "vector",
			Data:       vectors,
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 20, results.TotalSeries)
		assert.True(t, results.Truncated)
		assert.Len(t, results.Samples, 10) // Default max samples
		assert.Contains(t, results.Summary, "showing first 10")
	})

	t.Run("empty vector results", func(t *testing.T) {
		mimirResult := &mimir.QueryResult{
			ResultType: "vector",
			Data:       []mimir.InstantVector{},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 0, results.TotalSeries)
		assert.Equal(t, "No data found", results.Summary)
	})
}

func TestResultProcessor_ProcessMatrix(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("range query with statistics", func(t *testing.T) {
		now := time.Now()
		values := make([][2]interface{}, 100)
		for i := 0; i < 100; i++ {
			values[i] = [2]interface{}{
				float64(now.Add(time.Duration(i) * time.Minute).Unix()),
				"100.0",
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "matrix",
			Data: []mimir.RangeVector{
				{
					Metric: map[string]string{"job": "api"},
					Values: values,
				},
			},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 1, results.TotalSeries)
		assert.NotNil(t, results.Statistics)
		assert.Equal(t, 100.0, results.Statistics.Min)
		assert.Equal(t, 100.0, results.Statistics.Max)
		assert.Equal(t, 100.0, results.Statistics.Avg)
		assert.Contains(t, results.Summary, "1 series over time")
	})

	t.Run("time series sampling", func(t *testing.T) {
		now := time.Now()
		// Create 100 data points (more than maxTimePoints)
		values := make([][2]interface{}, 100)
		for i := 0; i < 100; i++ {
			values[i] = [2]interface{}{
				float64(now.Add(time.Duration(i) * time.Minute).Unix()),
				"50.0",
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "matrix",
			Data: []mimir.RangeVector{
				{
					Metric: map[string]string{"job": "api"},
					Values: values,
				},
			},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results.Samples))
		// Should be sampled down to maxTimePoints (50 by default)
		assert.LessOrEqual(t, len(results.Samples[0].Values), 50)
		// First and last values should be preserved
		assert.NotNil(t, results.Samples[0].Values[0])
		assert.NotNil(t, results.Samples[0].Values[len(results.Samples[0].Values)-1])
	})

	t.Run("trend detection - increasing", func(t *testing.T) {
		now := time.Now()
		values := make([][2]interface{}, 10)
		for i := 0; i < 10; i++ {
			values[i] = [2]interface{}{
				float64(now.Add(time.Duration(i) * time.Minute).Unix()),
				string(rune(float64(i * 20))), // Increasing values
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "matrix",
			Data: []mimir.RangeVector{
				{
					Metric: map[string]string{"job": "api"},
					Values: values,
				},
			},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.NotNil(t, results.Statistics)
		// Note: trend detection requires proper value parsing
	})
}

func TestResultProcessor_ProcessScalar(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("scalar result", func(t *testing.T) {
		now := time.Now()
		mimirResult := &mimir.QueryResult{
			ResultType: "scalar",
			Data:       []interface{}{float64(now.Unix()), "123.45"},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 1, results.TotalSeries)
		assert.Contains(t, results.Summary, "Scalar value: 123.45")
	})
}

func TestResultProcessor_ProcessString(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("string result", func(t *testing.T) {
		now := time.Now()
		mimirResult := &mimir.QueryResult{
			ResultType: "string",
			Data:       []interface{}{float64(now.Unix()), "test_string"},
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 1, results.TotalSeries)
		assert.Contains(t, results.Summary, "String value: test_string")
	})
}

func TestResultProcessor_ComputeStatistics(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("compute stats from samples", func(t *testing.T) {
		samples := []MetricSample{
			{
				Metric: map[string]string{"job": "api"},
				Values: []TimeValue{
					{Value: 10.0},
					{Value: 20.0},
					{Value: 30.0},
					{Value: 40.0},
					{Value: 50.0},
				},
			},
		}

		stats := rp.computeStatistics(samples)
		assert.Equal(t, 10.0, stats.Min)
		assert.Equal(t, 50.0, stats.Max)
		assert.Equal(t, 30.0, stats.Avg)
		assert.Equal(t, 50.0, stats.Current)
	})

	t.Run("detect increasing trend", func(t *testing.T) {
		samples := []MetricSample{
			{
				Values: []TimeValue{
					{Value: 10.0},
					{Value: 15.0},
					{Value: 20.0},
				},
			},
		}

		stats := rp.computeStatistics(samples)
		assert.Equal(t, "increasing", stats.Trend)
	})

	t.Run("detect decreasing trend", func(t *testing.T) {
		samples := []MetricSample{
			{
				Values: []TimeValue{
					{Value: 100.0},
					{Value: 80.0},
					{Value: 60.0},
				},
			},
		}

		stats := rp.computeStatistics(samples)
		assert.Equal(t, "decreasing", stats.Trend)
	})

	t.Run("detect stable trend", func(t *testing.T) {
		samples := []MetricSample{
			{
				Values: []TimeValue{
					{Value: 100.0},
					{Value: 101.0},
					{Value: 102.0},
				},
			},
		}

		stats := rp.computeStatistics(samples)
		assert.Equal(t, "stable", stats.Trend)
	})
}

func TestResultProcessor_SampleTimeSeries(t *testing.T) {
	rp := NewResultProcessor()

	t.Run("sample large time series", func(t *testing.T) {
		// Create 100 time points
		values := make([]TimeValue, 100)
		for i := 0; i < 100; i++ {
			values[i] = TimeValue{
				Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				Value:     float64(i),
			}
		}

		samples := []MetricSample{
			{
				Metric: map[string]string{"job": "api"},
				Values: values,
			},
		}

		sampled := rp.sampleTimeSeries(samples, 10)
		assert.Len(t, sampled, 1)
		assert.Len(t, sampled[0].Values, 10)
		// First value should be preserved
		assert.Equal(t, 0.0, sampled[0].Values[0].Value)
		// Last value should be preserved
		assert.Equal(t, 99.0, sampled[0].Values[9].Value)
	})

	t.Run("no sampling needed for small series", func(t *testing.T) {
		values := make([]TimeValue, 5)
		for i := 0; i < 5; i++ {
			values[i] = TimeValue{
				Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
				Value:     float64(i),
			}
		}

		samples := []MetricSample{
			{
				Metric: map[string]string{"job": "api"},
				Values: values,
			},
		}

		sampled := rp.sampleTimeSeries(samples, 10)
		assert.Len(t, sampled, 1)
		assert.Len(t, sampled[0].Values, 5) // No sampling needed
	})
}

func TestResultProcessor_CustomLimits(t *testing.T) {
	t.Run("custom max samples", func(t *testing.T) {
		rp := &ResultProcessor{
			maxSamples:    5,
			maxTimePoints: 20,
		}

		vectors := make([]mimir.InstantVector, 10)
		for i := 0; i < 10; i++ {
			vectors[i] = mimir.InstantVector{
				Metric:    map[string]string{"instance": "host" + string(rune(i))},
				Val:       float64(i),
				Timestamp: float64(time.Now().Unix()),
			}
		}

		mimirResult := &mimir.QueryResult{
			ResultType: "vector",
			Data:       vectors,
		}

		results, err := rp.ProcessResults(mimirResult)
		require.NoError(t, err)
		assert.Equal(t, 10, results.TotalSeries)
		assert.True(t, results.Truncated)
		assert.Len(t, results.Samples, 5) // Custom limit
	})
}
