package zscore_test

import (
	"testing"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/anomaly/zscore"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/stretchr/testify/assert"
)

type DummyExtractor struct{}

func (d *DummyExtractor) Extract(s *nodes.NodeTimePeriodSummary) (float64, error) {
	return float64(len(s.Events)), nil
}

func TestDetector(t *testing.T) {
	detector := zscore.NewDetector(zscore.ZScore95, &DummyExtractor{})
	done := make(chan struct{})
	summaries := make(chan *nodes.NodeTimePeriodSummary)

	go func() {
		assert.NoError(t, detector.Train(summaries, done))
	}()

	// Push a lot of two event node summaries so that when we test
	// with a 2+ event summary, an anomaly is detected.
	for i := 0; i < 100; i++ {
		summaries <- &nodes.NodeTimePeriodSummary{
			Node: &nodes.Node{},
			Events: []*nodes.NodeEvent{
				&nodes.NodeEvent{},
				&nodes.NodeEvent{},
			},
		}
	}

	close(summaries)
	<-done

	t.Log(detector)

	t.Run("anomaly", func(t *testing.T) {
		// This is an anomaly since it has more than two events.
		anomaly := &nodes.NodeTimePeriodSummary{
			Node: &nodes.Node{},
			Events: []*nodes.NodeEvent{
				&nodes.NodeEvent{},
				&nodes.NodeEvent{},
				&nodes.NodeEvent{},
				&nodes.NodeEvent{},
			},
		}
		isAnomaly, err := detector.IsAnomaly(anomaly)
		if assert.NoError(t, err) {
			assert.True(t, isAnomaly)
		}
	})

	t.Run("non-anomaly", func(t *testing.T) {
		// This is not an anomaly since it has exactly two events,
		// which is the same number as the training set.
		nonAnomaly := &nodes.NodeTimePeriodSummary{
			Node: &nodes.Node{},
			Events: []*nodes.NodeEvent{
				&nodes.NodeEvent{},
				&nodes.NodeEvent{},
			},
		}
		isAnomaly, err := detector.IsAnomaly(nonAnomaly)
		if assert.NoError(t, err) {
			assert.False(t, isAnomaly)
		}
	})
}
