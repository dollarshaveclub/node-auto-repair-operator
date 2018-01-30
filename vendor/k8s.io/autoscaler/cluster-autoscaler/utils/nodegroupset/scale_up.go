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

package nodegroupset

import (
	"fmt"
	"sort"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"

	"github.com/golang/glog"
)

// ScaleUpInfo contains information about planned scale-up of a single NodeGroup
type ScaleUpInfo struct {
	// Group is the group to be scaled-up
	Group cloudprovider.NodeGroup
	// CurrentSize is the current size of the Group
	CurrentSize int
	// NewSize is the size the Group will be scaled-up to
	NewSize int
	// MaxSize is the maximum allowed size of the Group
	MaxSize int
}

// String is used for printing ScaleUpInfo for logging, etc
func (s ScaleUpInfo) String() string {
	return fmt.Sprintf("{%v %v->%v (max: %v)}", s.Group.Id(), s.CurrentSize, s.NewSize, s.MaxSize)
}

// BalanceScaleUpBetweenGroups distributes a given number of nodes between
// given set of NodeGroups. The nodes are added to smallest group first, trying
// to make the group sizes as evenly balanced as possible.
//
// Returns ScaleUpInfos for groups that need to be resized.
//
// MaxSize of each group will be respected. If newNodes > total free capacity
// of all NodeGroups it will be capped to total capacity. In particular if all
// group already have MaxSize, empty list will be returned.
func BalanceScaleUpBetweenGroups(groups []cloudprovider.NodeGroup, newNodes int) ([]ScaleUpInfo, errors.AutoscalerError) {
	if len(groups) == 0 {
		return []ScaleUpInfo{}, errors.NewAutoscalerError(
			errors.InternalError, "Can't balance scale up between 0 groups")
	}

	// get all data from cloudprovider, build data structure
	scaleUpInfos := make([]ScaleUpInfo, 0)
	totalCapacity := 0
	for _, ng := range groups {
		currentSize, err := ng.TargetSize()
		if err != nil {
			return []ScaleUpInfo{}, errors.NewAutoscalerError(
				errors.CloudProviderError,
				"failed to get node group size: %v", err)
		}
		maxSize := ng.MaxSize()
		if currentSize == maxSize {
			// group already maxed, ignore it
			continue
		}
		info := ScaleUpInfo{
			Group:       ng,
			CurrentSize: currentSize,
			NewSize:     currentSize,
			MaxSize:     maxSize}
		scaleUpInfos = append(scaleUpInfos, info)
		totalCapacity += maxSize - currentSize
	}
	if totalCapacity < newNodes {
		glog.V(2).Infof("Requested scale-up (%v) exceeds node group set capacity, capping to %v", newNodes, totalCapacity)
		newNodes = totalCapacity
	}

	// The actual balancing algorithm.
	// Sort the node groups by current size and just loop over nodes adding
	// to smallest group. If a group hits max size remove it from the list
	// (by moving it to start of the list and increasing startIndex).
	//
	// In each iteration we either allocate one node, or 'remove' a maxed out
	// node group, so this will terminate in O(#nodes + #node groups) steps.
	// We already know that newNodes <= total capacity, so we don't have to
	// worry about accidentally removing all node groups while we still
	// have nodes to allocate.
	//
	// Loop invariants:
	// 1. i < startIndex -> scaleUpInfos[i].CurrentSize == scaleUpInfos[i].MaxSize
	// 2. i >= startIndex -> scaleUpInfos[i].CurrentSize < scaleUpInfos[i].MaxSize
	// 3. startIndex <= currentIndex < len(scaleUpInfos)
	// 4. currentIndex <= i < j -> scaleUpInfos[i].CurrentSize <= scaleUpInfos[j].CurrentSize
	// 5. startIndex <= i < j < currentIndex -> scaleUpInfos[i].CurrentSize == scaleUpInfos[j].CurrentSize
	// 6. startIndex <= i < currentIndex <= j -> scaleUpInfos[i].CurrentSize <= scaleUpInfos[j].CurrentSize + 1
	sort.Slice(scaleUpInfos, func(i, j int) bool {
		return scaleUpInfos[i].CurrentSize < scaleUpInfos[j].CurrentSize
	})
	startIndex := 0
	currentIndex := 0
	for newNodes > 0 {
		currentInfo := &scaleUpInfos[currentIndex]

		if currentInfo.NewSize < currentInfo.MaxSize {
			// Add a node to group on currentIndex
			currentInfo.NewSize++
			newNodes--
		} else {
			// Group on currentIndex is full. Remove it from the array.
			// Removing is done by swapping the group with the first
			// group still in array and moving the start of the array.
			// Every group between startIndex and currentIndex has the
			// same size, so we can swap without breaking ordering.
			scaleUpInfos[startIndex], scaleUpInfos[currentIndex] = scaleUpInfos[currentIndex], scaleUpInfos[startIndex]
			startIndex++
		}

		// Update currentIndex.
		// If we removed a group in this loop currentIndex may be equal to startIndex-1,
		// in which case both branches of below if will make currentIndex == startIndex.
		if currentIndex < len(scaleUpInfos)-1 && currentInfo.NewSize > scaleUpInfos[currentIndex+1].NewSize {
			// Next group has exactly one less node, than current one.
			// We will increase it in next iteration.
			currentIndex++
		} else {
			// We reached end of array, or a group larger than the current one.
			// All groups from startIndex to currentIndex have the same size.
			// So we're moving to the beginning of array to loop over all of
			// them once again.
			currentIndex = startIndex
		}
	}

	// Filter out groups that haven't changed size
	result := make([]ScaleUpInfo, 0)
	for _, info := range scaleUpInfos {
		if info.NewSize != info.CurrentSize {
			result = append(result, info)
		}
	}

	return result, nil
}
