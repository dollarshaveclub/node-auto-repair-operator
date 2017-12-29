package extractors

import "github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"

// FlappingNode is a FeatureExtractor that tries to highlight
// instability by capturing the number of times that a Kubernetes node
// alternates between a Ready/NotReady status.
type FlappingNode struct{}

func (f *FlappingNode) Extract(ns *nodes.NodeTimePeriodSummary) (float64, error) {
	return 0, nil
}
