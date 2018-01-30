/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"time"

	"github.com/golang/glog"
	k8sapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/vertical-pod-autoscaler/recommender/model"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	resourceclient "k8s.io/metrics/pkg/client/clientset_generated/clientset/typed/metrics/v1beta1"
)

// ContainerMetricsSnapshot contains information about usage of certain container within defined time window.
type ContainerMetricsSnapshot struct {
	// ID identifies a specific container those metrics are coming from.
	ID model.ContainerID
	// End time of the measurement interval.
	SnapshotTime time.Time
	// Duration of the measurement interval, which is [SnapshotTime - SnapshotWindow, SnapshotTime].
	SnapshotWindow time.Duration
	// Actual usage of the resources over the measurement interval.
	Usage model.Resources
}

// MetricsClient provides simple metrics on resources usage on containter level.
type MetricsClient interface {
	// GetContainersMetrics returns an array of ContainerMetricsSnapshots,
	// representing resource usage for every running container in the cluster
	GetContainersMetrics() ([]*ContainerMetricsSnapshot, error)
}

type metricsClient struct {
	metricsGetter resourceclient.PodMetricsesGetter
}

// NewMetricsClient creates new instance of MetricsClient, which is used by recommender.
// It requires an instance of PodMetricsesGetter, which is used for underlying communication with metrics server.
func NewMetricsClient(metricsGetter resourceclient.PodMetricsesGetter) MetricsClient {
	return &metricsClient{
		metricsGetter: metricsGetter,
	}
}

func (c *metricsClient) GetContainersMetrics() ([]*ContainerMetricsSnapshot, error) {
	var metricsSnapshots []*ContainerMetricsSnapshot

	podMetricsInterface := c.metricsGetter.PodMetricses(api.NamespaceAll)
	podMetricsList, err := podMetricsInterface.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	glog.V(3).Infof("%v podMetrics retrived for all namespaces", len(podMetricsList.Items))
	for _, podMetrics := range podMetricsList.Items {
		metricsSnapshotsForPod := createContainerMetricsSnapshots(podMetrics)
		metricsSnapshots = append(metricsSnapshots, metricsSnapshotsForPod...)
	}

	return metricsSnapshots, nil
}

func createContainerMetricsSnapshots(podMetrics v1beta1.PodMetrics) []*ContainerMetricsSnapshot {
	snapshots := make([]*ContainerMetricsSnapshot, len(podMetrics.Containers))
	for i, containerMetrics := range podMetrics.Containers {
		snapshots[i] = newContainerMetricsSnapshot(containerMetrics, podMetrics)
	}
	return snapshots
}

func newContainerMetricsSnapshot(containerMetrics v1beta1.ContainerMetrics, podMetrics v1beta1.PodMetrics) *ContainerMetricsSnapshot {
	usage := calculateUsage(containerMetrics.Usage)

	return &ContainerMetricsSnapshot{
		ID: model.ContainerID{
			ContainerName: containerMetrics.Name,
			PodID: model.PodID{
				Namespace: podMetrics.Namespace,
				PodName:   podMetrics.Name,
			},
		},
		Usage:          usage,
		SnapshotTime:   podMetrics.Timestamp.Time,
		SnapshotWindow: podMetrics.Window.Duration,
	}
}

func calculateUsage(containerUsage k8sapiv1.ResourceList) model.Resources {
	cpuQuantity := containerUsage[k8sapiv1.ResourceCPU]
	cpuMillicores := cpuQuantity.MilliValue()

	memoryQuantity := containerUsage[k8sapiv1.ResourceMemory]
	memoryBytes := memoryQuantity.Value()

	return model.Resources{
		model.ResourceCPU:    model.ResourceAmount(cpuMillicores),
		model.ResourceMemory: model.ResourceAmount(memoryBytes),
	}
}
