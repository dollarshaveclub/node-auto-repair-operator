package zscore_test

import (
	"testing"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/zscore"
	"github.com/stretchr/testify/assert"
)

type DummyExtractor struct{}

func (d *DummyExtractor) String() string {
	return "DummyExtractor"
}

func (d *DummyExtractor) Extract(s *naro.NodeTimePeriodSummary) (float64, error) {
	return float64(len(s.Events)), nil
}

func TestDetector(t *testing.T) {
	detector := zscore.NewDetector(zscore.ZScore95, &DummyExtractor{})
	var summaries []*naro.NodeTimePeriodSummary

	// Push a lot of two event node summaries so that when we test
	// with a 2+ event summary, an anomaly is detected.
	for i := 0; i < 100; i++ {
		summaries = append(summaries, &naro.NodeTimePeriodSummary{
			Node: &naro.Node{},
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{},
				&naro.NodeEvent{},
			},
		})
	}

	assert.NoError(t, detector.Train(summaries))

	t.Log(detector)

	t.Run("anomaly", func(t *testing.T) {
		// This is an anomaly since it has more than two events.
		anomaly := &naro.NodeTimePeriodSummary{
			Node: &naro.Node{},
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{},
				&naro.NodeEvent{},
				&naro.NodeEvent{},
				&naro.NodeEvent{},
			},
		}
		isAnomaly, _, err := detector.IsAnomalous(anomaly)
		if assert.NoError(t, err) {
			assert.True(t, isAnomaly)
		}
	})

	t.Run("non-anomaly", func(t *testing.T) {
		// This is not an anomaly since it has exactly two events,
		// which is the same number as the training set.
		nonAnomaly := &naro.NodeTimePeriodSummary{
			Node: &naro.Node{},
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{},
				&naro.NodeEvent{},
			},
		}
		isAnomaly, _, err := detector.IsAnomalous(nonAnomaly)
		if assert.NoError(t, err) {
			assert.False(t, isAnomaly)
		}
	})
}
