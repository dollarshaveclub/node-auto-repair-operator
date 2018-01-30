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

package logic

import (
	"k8s.io/autoscaler/vertical-pod-autoscaler/recommender/model"
	"k8s.io/autoscaler/vertical-pod-autoscaler/recommender/util"
)

// PodResourceRecommender computes resource recommendation for a Vpa object.
type PodResourceRecommender interface {
	GetRecommendedPodResources(vpa *model.Vpa) RecommendedPodResources
}

// RecommendedPodResources is a Map from container name to recommended resources.
type RecommendedPodResources map[string]RecommendedContainerResources

// RecommendedContainerResources is the recommendation of resources for a
// container.
type RecommendedContainerResources struct {
	// Recommended optimal amount of resources.
	Target model.Resources
	// Recommended minimum amount of resources.
	MinRecommended model.Resources
	// Recommended maximum amount of resources.
	MaxRecommended model.Resources
}

type podResourceRecommender struct {
	targetEstimator     ResourceEstimator
	lowerBoundEstimator ResourceEstimator
	upperBoundEstimator ResourceEstimator
}

// NewPodResourceRecommender returns a new PodResourceRecommender which is an
// aggregation of three ResourceEstimators, one for each of the target, min
// and max recommended resources.
func NewPodResourceRecommender(
	targetEstimator ResourceEstimator,
	lowerBoundEstimator ResourceEstimator,
	upperBoundEstimator ResourceEstimator) PodResourceRecommender {
	return &podResourceRecommender{targetEstimator, lowerBoundEstimator, upperBoundEstimator}
}

// Returns recommended resources for a given Vpa object.
func (r *podResourceRecommender) GetRecommendedPodResources(vpa *model.Vpa) RecommendedPodResources {
	aggregateContainerStateMap := buildAggregateContainerStateMap(&vpa.Pods)
	var recommendation RecommendedPodResources = make(RecommendedPodResources)
	for containerName, aggregatedContainerState := range aggregateContainerStateMap {
		recommendation[containerName] = r.getRecommendedContainerResources(aggregatedContainerState)
	}
	return recommendation
}

// AggregateContainerState holds input signals aggregated from a set of containers.
// It can be used as an input to compute the recommendation.
type AggregateContainerState struct {
	aggregateCPUUsage    util.Histogram
	aggregateMemoryPeaks util.Histogram
}

// Merges the state of an individual container into AggregateContainerState.
func (a *AggregateContainerState) mergeContainerState(container *model.ContainerState) {
	a.aggregateCPUUsage.Merge(&container.CPUUsage)
	for _, memoryPeak := range container.MemoryUsagePeaks.Contents() {
		// TODO: may use exponential decay here.
		a.aggregateMemoryPeaks.AddSample(float64(memoryPeak), 1.0)
	}
}

// Returns a new, empty AggregateContainerState.
func newAggregateContainerState() *AggregateContainerState {
	return &AggregateContainerState{
		util.NewHistogram(model.CPUHistogramOptions),
		util.NewHistogram(model.MemoryHistogramOptions),
	}
}

// Takes a set of pods and groups their containers by name.
// Returns a map from the container name to the AggregateContainerState.
func buildAggregateContainerStateMap(pods *map[model.PodID]*model.PodState) map[string]*AggregateContainerState {
	aggregateContainerStateMap := make(map[string]*AggregateContainerState)
	for _, pod := range *pods {
		for containerName, container := range pod.Containers {
			aggregateContainerState, isInitialized := aggregateContainerStateMap[containerName]
			if !isInitialized {
				aggregateContainerState = newAggregateContainerState()
				aggregateContainerStateMap[containerName] = aggregateContainerState
			}
			aggregateContainerState.mergeContainerState(container)
		}
	}
	return aggregateContainerStateMap
}

// Takes AggregateContainerState and returns a container recommendation.
func (r *podResourceRecommender) getRecommendedContainerResources(s *AggregateContainerState) RecommendedContainerResources {
	return RecommendedContainerResources{
		r.targetEstimator.GetResourceEstimation(s),
		r.lowerBoundEstimator.GetResourceEstimation(s),
		r.upperBoundEstimator.GetResourceEstimation(s),
	}
}
