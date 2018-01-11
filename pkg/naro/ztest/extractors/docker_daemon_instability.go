package extractors

import (
	"github.com/Sirupsen/logrus"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
)

// DockerDaemonInstability is a zscore.FeatureExtractor that tries to
// highlight Docker daemon instability on a Kubernetes node by
// capturing the number of times instability is detected.
type DockerDaemonInstability struct{}

// NewDockerDaemonInstability returns a new instance of
// DockerDaemonInstability.
func NewDockerDaemonInstability() *DockerDaemonInstability {
	return &DockerDaemonInstability{}
}

// String returns the string representation of
// DockerDaemonInstability.
func (f *DockerDaemonInstability) String() string {
	return "DockerDaemonInstability"
}

// Extract returns the number of times Docker daemon instability is
// detected within the naro.NodeTimePeriodSummary.
func (f *DockerDaemonInstability) Extract(ns *naro.NodeTimePeriodSummary) (float64, error) {
	logrus.Debugf("DockerDaemonInstability: extracting feature from %d naro.NodeEvents",
		len(ns.Events))

	instabilityPeriods := 0
	unstable := false

	// This loop iterates through all events looking for periods
	// of time where a node experienced a NodeNotReady event
	// followed by a ContainerGCFailed event. Via experimentation,
	// we found that these events align with Docker daemon issues.
	for _, event := range ns.Events {
		if event.Reason == "NodeNotReady" &&
			event.SourceComponent == "controllermanager" {
			unstable = true
			continue
		}
		if unstable {
			// ContainerGCFailed confirms that the
			// situation is a Docker daemon issue.
			if event.Reason == "ContainerGCFailed" &&
				event.SourceComponent == "kubelet" {
				instabilityPeriods++
				unstable = false
				continue
			}

			// If the node is now ready, we did not detect
			// a Docker daemon issue, so we start from
			// scratch.
			if event.Reason == "NodeReady" &&
				event.SourceComponent == "kubelet" {
				unstable = false
				continue
			}
		}
	}

	return float64(instabilityPeriods), nil
}
