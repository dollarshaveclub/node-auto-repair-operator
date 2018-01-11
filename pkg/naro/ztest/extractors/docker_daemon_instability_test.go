package extractors_test

import (
	"testing"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/ztest/extractors"
	"github.com/stretchr/testify/assert"
)

func TestDockerDaemonInstabilityExtract(t *testing.T) {
	extractor := extractors.NewDockerDaemonInstability()

	t.Run("nothing", func(t *testing.T) {
		tps := &naro.NodeTimePeriodSummary{}
		feature, err := extractor.Extract(tps)
		if assert.NoError(t, err) {
			assert.Zero(t, feature)
		}
	})

	t.Run("partial", func(t *testing.T) {
		tps := &naro.NodeTimePeriodSummary{
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{
					Reason:          "NodeNotReady",
					SourceComponent: "controllermanager",
				},
			},
		}
		feature, err := extractor.Extract(tps)
		if assert.NoError(t, err) {
			assert.Zero(t, feature)
		}
	})

	t.Run("full", func(t *testing.T) {
		tps := &naro.NodeTimePeriodSummary{
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{
					Reason:          "NodeNotReady",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "ContainerGCFailed",
					SourceComponent: "kubelet",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
			},
		}
		feature, err := extractor.Extract(tps)
		if assert.NoError(t, err) {
			assert.Equal(t, float64(1), feature)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		tps := &naro.NodeTimePeriodSummary{
			Events: []*naro.NodeEvent{
				&naro.NodeEvent{
					Reason:          "NodeNotReady",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "ContainerGCFailed",
					SourceComponent: "kubelet",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "NodeNotReady",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
				&naro.NodeEvent{
					Reason:          "ContainerGCFailed",
					SourceComponent: "kubelet",
				},
				&naro.NodeEvent{
					Reason:          "Test",
					SourceComponent: "controllermanager",
				},
			},
		}
		feature, err := extractor.Extract(tps)
		if assert.NoError(t, err) {
			assert.Equal(t, float64(2), feature)
		}
	})
}
