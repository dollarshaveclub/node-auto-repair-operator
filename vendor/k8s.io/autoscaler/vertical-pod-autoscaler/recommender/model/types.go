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

package model

import (
	"github.com/golang/glog"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceName represents the name of the resource monitored by recommender.
type ResourceName string

// ResourceAmount represents quantity of a certain resource within a container.
// Note this keeps CPU in millicores (which is not a standard unit in APIs)
// and memory in bytes.
type ResourceAmount int64

// Resources is a map from resource name to the corresponding ResourceAmount.
type Resources map[ResourceName]ResourceAmount

const (
	// ResourceCPU represents CPU in millicores (1core = 1000millicores).
	ResourceCPU ResourceName = "cpu"
	// ResourceMemory represents memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024).
	ResourceMemory ResourceName = "memory"
)

// CPUAmountFromCores converts CPU cores to a ResourceAmount.
func CPUAmountFromCores(cores float64) ResourceAmount {
	return ResourceAmount(cores * 1000.0)
}

// CoresFromCPUAmount converts ResourceAmount to number of cores expressed as float64.
func CoresFromCPUAmount(cpuAmount ResourceAmount) float64 {
	return float64(cpuAmount) / 1000.0
}

// QuantityFromCPUAmount converts CPU ResourceAmount to a resource.Quantity.
func QuantityFromCPUAmount(cpuAmount ResourceAmount) resource.Quantity {
	return *resource.NewScaledQuantity(int64(cpuAmount), -3)
}

// MemoryAmountFromBytes converts memory bytes to a ResourceAmount.
func MemoryAmountFromBytes(bytes float64) ResourceAmount {
	return ResourceAmount(bytes)
}

// BytesFromMemoryAmount converts ResourceAmount to number of bytes expressed as float64.
func BytesFromMemoryAmount(memoryAmount ResourceAmount) float64 {
	return float64(memoryAmount)
}

// QuantityFromMemoryAmount converts memory ResourceAmount to a resource.Quantity.
func QuantityFromMemoryAmount(memoryAmount ResourceAmount) resource.Quantity {
	return *resource.NewScaledQuantity(int64(memoryAmount), 0)
}

// ResourcesAsResourceList converts internal Resources representation to ResourcesList.
func ResourcesAsResourceList(resources Resources) apiv1.ResourceList {
	result := make(apiv1.ResourceList)
	for key, resourceAmount := range resources {
		var newKey apiv1.ResourceName
		var quantity resource.Quantity
		switch key {
		case ResourceCPU:
			newKey = apiv1.ResourceCPU
			quantity = QuantityFromCPUAmount(resourceAmount)
		case ResourceMemory:
			newKey = apiv1.ResourceMemory
			quantity = QuantityFromMemoryAmount(resourceAmount)
		default:
			glog.Errorf("Cannot translate %v resource name", key)
			continue
		}
		result[newKey] = quantity
	}
	return result
}

// PodID contains information needed to identify a Pod within a cluster.
type PodID struct {
	// Namespaces where the Pod is defined.
	Namespace string
	// PodName is the name of the pod unique within a namespace.
	PodName string
}

// ContainerID contains information needed to identify a Container within a cluster.
type ContainerID struct {
	PodID
	// ContainerName is the name of the container, unique within a pod.
	ContainerName string
}

// VpaID contains information needed to identify a VPA API object within a cluster.
type VpaID struct {
	Namespace string
	VpaName   string
}
