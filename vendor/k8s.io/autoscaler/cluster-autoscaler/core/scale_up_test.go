/*
Copyright 2016 The Kubernetes Authors.

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

package core

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	testprovider "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/test"
	"k8s.io/autoscaler/cluster-autoscaler/clusterstate"
	"k8s.io/autoscaler/cluster-autoscaler/clusterstate/utils"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/estimator"
	"k8s.io/autoscaler/cluster-autoscaler/expander/random"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	kube_util "k8s.io/autoscaler/cluster-autoscaler/utils/kubernetes"
	. "k8s.io/autoscaler/cluster-autoscaler/utils/test"

	apiv1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	kube_record "k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/stretchr/testify/assert"
)

type nodeConfig struct {
	name   string
	cpu    int64
	memory int64
	ready  bool
	group  string
}

type podConfig struct {
	name   string
	cpu    int64
	memory int64
	node   string
}

type scaleTestConfig struct {
	nodes                []nodeConfig
	pods                 []podConfig
	extraPods            []podConfig
	expectedScaleUp      string
	expectedScaleUpGroup string
	expectedScaleDowns   []string
	options              AutoscalingOptions
}

var defaultOptions = AutoscalingOptions{
	EstimatorName:  estimator.BinpackingEstimatorName,
	MaxCoresTotal:  config.DefaultMaxClusterCores,
	MaxMemoryTotal: config.DefaultMaxClusterMemory,
	MinCoresTotal:  0,
	MinMemoryTotal: 0,
}

func TestScaleUpOK(t *testing.T) {
	config := &scaleTestConfig{
		nodes: []nodeConfig{
			{"n1", 100, 100, true, "ng1"},
			{"n2", 1000, 1000, true, "ng2"},
		},
		pods: []podConfig{
			{"p1", 80, 0, "n1"},
			{"p2", 800, 0, "n2"},
		},
		extraPods: []podConfig{
			{"p-new", 500, 0, ""},
		},
		expectedScaleUp:      "ng2-1",
		expectedScaleUpGroup: "ng2",
		options:              defaultOptions,
	}

	simpleScaleUpTest(t, config)
}

func TestScaleUpMaxCoresLimitHit(t *testing.T) {
	options := defaultOptions
	options.MaxCoresTotal = 9
	config := &scaleTestConfig{
		nodes: []nodeConfig{
			{"n1", 2000, 100, true, "ng1"},
			{"n2", 4000, 1000, true, "ng2"},
		},
		pods: []podConfig{
			{"p1", 1000, 0, "n1"},
			{"p2", 3000, 0, "n2"},
		},
		extraPods: []podConfig{
			{"p-new-1", 2000, 0, ""},
			{"p-new-2", 2000, 0, ""},
		},
		expectedScaleUp:      "ng1-1",
		expectedScaleUpGroup: "ng1",
		options:              options,
	}

	simpleScaleUpTest(t, config)
}

const MB = 1024 * 1024

func TestScaleUpMaxMemoryLimitHit(t *testing.T) {
	options := defaultOptions
	options.MaxMemoryTotal = 1300 // set in mb
	config := &scaleTestConfig{
		nodes: []nodeConfig{
			{"n1", 2000, 100 * MB, true, "ng1"},
			{"n2", 4000, 1000 * MB, true, "ng2"},
		},
		pods: []podConfig{
			{"p1", 1000, 0, "n1"},
			{"p2", 3000, 0, "n2"},
		},
		extraPods: []podConfig{
			{"p-new-1", 2000, 100 * MB, ""},
			{"p-new-2", 2000, 100 * MB, ""},
			{"p-new-3", 2000, 100 * MB, ""},
		},
		expectedScaleUp:      "ng1-2",
		expectedScaleUpGroup: "ng1",
		options:              options,
	}

	simpleScaleUpTest(t, config)
}

func TestScaleUpCapToMaxTotalNodesLimit(t *testing.T) {
	options := defaultOptions
	options.MaxNodesTotal = 3
	config := &scaleTestConfig{
		nodes: []nodeConfig{
			{"n1", 2000, 100 * MB, true, "ng1"},
			{"n2", 4000, 1000 * MB, true, "ng2"},
		},
		pods: []podConfig{
			{"p1", 1000, 0, "n1"},
			{"p2", 3000, 0, "n2"},
		},
		extraPods: []podConfig{
			{"p-new-1", 4000, 100 * MB, ""},
			{"p-new-2", 4000, 100 * MB, ""},
			{"p-new-3", 4000, 100 * MB, ""},
		},
		expectedScaleUp:      "ng2-1",
		expectedScaleUpGroup: "ng2",
		options:              options,
	}

	simpleScaleUpTest(t, config)
}

func simpleScaleUpTest(t *testing.T, config *scaleTestConfig) {
	expandedGroups := make(chan string, 10)
	fakeClient := &fake.Clientset{}

	groups := make(map[string][]*apiv1.Node)
	nodes := make([]*apiv1.Node, len(config.nodes))
	for i, n := range config.nodes {
		node := BuildTestNode(n.name, n.cpu, n.memory)
		SetNodeReadyState(node, n.ready, time.Now())
		nodes[i] = node
		groups[n.group] = append(groups[n.group], node)
	}

	pods := make(map[string][]apiv1.Pod)
	for _, p := range config.pods {
		pod := *BuildTestPod(p.name, p.cpu, p.memory)
		pod.Spec.NodeName = p.node
		pods[p.node] = append(pods[p.node], pod)
	}

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		for _, node := range nodes {
			if strings.Contains(fieldstring, node.Name) {
				return true, &apiv1.PodList{Items: pods[node.Name]}, nil
			}
		}
		return true, nil, fmt.Errorf("Failed to list: %v", list)
	})

	provider := testprovider.NewTestCloudProvider(func(nodeGroup string, increase int) error {
		expandedGroups <- fmt.Sprintf("%s-%d", nodeGroup, increase)
		return nil
	}, nil)

	for name, nodesInGroup := range groups {
		provider.AddNodeGroup(name, 1, 10, len(nodesInGroup))
		for _, n := range nodesInGroup {
			provider.AddNode(name, n)
		}
	}

	resourceLimiter := cloudprovider.NewResourceLimiter(
		map[string]int64{cloudprovider.ResourceNameCores: config.options.MinCoresTotal, cloudprovider.ResourceNameMemory: config.options.MinMemoryTotal},
		map[string]int64{cloudprovider.ResourceNameCores: config.options.MaxCoresTotal, cloudprovider.ResourceNameMemory: config.options.MaxMemoryTotal})
	provider.SetResourceLimiter(resourceLimiter)

	assert.NotNil(t, provider)

	fakeRecorder := kube_record.NewFakeRecorder(5)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", kube_record.NewFakeRecorder(5), false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)

	clusterState.UpdateNodes(nodes, time.Now())

	context := &AutoscalingContext{
		AutoscalingOptions:   config.options,
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}

	extraPods := make([]*apiv1.Pod, len(config.extraPods))
	for i, p := range config.extraPods {
		pod := BuildTestPod(p.name, p.cpu, p.memory)
		extraPods[i] = pod
	}

	result, err := ScaleUp(context, extraPods, nodes, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	assert.True(t, result)

	assert.Equal(t, config.expectedScaleUp, getStringFromChan(expandedGroups))

	nodeEventSeen := false
	for eventsLeft := true; eventsLeft; {
		select {
		case event := <-fakeRecorder.Events:
			if strings.Contains(event, "TriggeredScaleUp") && strings.Contains(event, config.expectedScaleUpGroup) {
				nodeEventSeen = true
			}
			assert.NotRegexp(t, regexp.MustCompile("NotTriggerScaleUp"), event)
		default:
			eventsLeft = false
		}
	}
	assert.True(t, nodeEventSeen)
}

func TestScaleUpNodeComingNoScale(t *testing.T) {
	n1 := BuildTestNode("n1", 100, 1000)
	SetNodeReadyState(n1, true, time.Now())
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Now())

	p1 := BuildTestPod("p1", 80, 0)
	p2 := BuildTestPod("p2", 800, 0)
	p1.Spec.NodeName = "n1"
	p2.Spec.NodeName = "n2"

	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		if strings.Contains(fieldstring, "n1") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p1}}, nil
		}
		if strings.Contains(fieldstring, "n2") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
		}
		return true, nil, fmt.Errorf("Failed to list: %v", list)
	})

	provider := testprovider.NewTestCloudProvider(func(nodeGroup string, increase int) error {
		t.Fatalf("No expansion is expected, but increased %s by %d", nodeGroup, increase)
		return nil
	}, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNodeGroup("ng2", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng2", n2)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)
	clusterState.RegisterScaleUp(&clusterstate.ScaleUpRequest{
		NodeGroupName:   "ng2",
		Increase:        1,
		Time:            time.Now(),
		ExpectedAddTime: time.Now().Add(5 * time.Minute),
	})
	clusterState.UpdateNodes([]*apiv1.Node{n1, n2}, time.Now())

	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			EstimatorName:  estimator.BinpackingEstimatorName,
			MaxCoresTotal:  config.DefaultMaxClusterCores,
			MaxMemoryTotal: config.DefaultMaxClusterMemory,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}
	p3 := BuildTestPod("p-new", 550, 0)

	result, err := ScaleUp(context, []*apiv1.Pod{p3}, []*apiv1.Node{n1, n2}, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	// A node is already coming - no need for scale up.
	assert.False(t, result)
}

func TestScaleUpNodeComingHasScale(t *testing.T) {
	n1 := BuildTestNode("n1", 100, 1000)
	SetNodeReadyState(n1, true, time.Now())
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Now())

	p1 := BuildTestPod("p1", 80, 0)
	p2 := BuildTestPod("p2", 800, 0)
	p1.Spec.NodeName = "n1"
	p2.Spec.NodeName = "n2"

	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		if strings.Contains(fieldstring, "n1") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p1}}, nil
		}
		if strings.Contains(fieldstring, "n2") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
		}
		return true, nil, fmt.Errorf("Failed to list: %v", list)
	})

	expandedGroups := make(chan string, 10)
	provider := testprovider.NewTestCloudProvider(func(nodeGroup string, increase int) error {
		expandedGroups <- fmt.Sprintf("%s-%d", nodeGroup, increase)
		return nil
	}, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNodeGroup("ng2", 1, 10, 2)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng2", n2)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)
	clusterState.RegisterScaleUp(&clusterstate.ScaleUpRequest{
		NodeGroupName:   "ng2",
		Increase:        1,
		Time:            time.Now(),
		ExpectedAddTime: time.Now().Add(5 * time.Minute),
	})
	clusterState.UpdateNodes([]*apiv1.Node{n1, n2}, time.Now())

	context := &AutoscalingContext{
		AutoscalingOptions:   defaultOptions,
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}
	p3 := BuildTestPod("p-new", 550, 0)

	result, err := ScaleUp(context, []*apiv1.Pod{p3, p3}, []*apiv1.Node{n1, n2}, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	// Twho nodes needed but one node is already coming, so it should increase by one.
	assert.True(t, result)
	assert.Equal(t, "ng2-1", getStringFromChan(expandedGroups))
}

func TestScaleUpUnhealthy(t *testing.T) {
	n1 := BuildTestNode("n1", 100, 1000)
	SetNodeReadyState(n1, true, time.Now())
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Now())

	p1 := BuildTestPod("p1", 80, 0)
	p2 := BuildTestPod("p2", 800, 0)
	p1.Spec.NodeName = "n1"
	p2.Spec.NodeName = "n2"

	fakeClient := &fake.Clientset{}
	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		if strings.Contains(fieldstring, "n1") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p1}}, nil
		}
		if strings.Contains(fieldstring, "n2") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
		}
		return true, nil, fmt.Errorf("Failed to list: %v", list)
	})

	provider := testprovider.NewTestCloudProvider(func(nodeGroup string, increase int) error {
		t.Fatalf("No expansion is expected, but increased %s by %d", nodeGroup, increase)
		return nil
	}, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNodeGroup("ng2", 1, 10, 5)
	provider.AddNode("ng1", n1)
	provider.AddNode("ng2", n2)

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)
	clusterState.UpdateNodes([]*apiv1.Node{n1, n2}, time.Now())
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			EstimatorName:  estimator.BinpackingEstimatorName,
			MaxCoresTotal:  config.DefaultMaxClusterCores,
			MaxMemoryTotal: config.DefaultMaxClusterMemory,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}
	p3 := BuildTestPod("p-new", 550, 0)

	result, err := ScaleUp(context, []*apiv1.Pod{p3}, []*apiv1.Node{n1, n2}, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	// Node group is unhealthy.
	assert.False(t, result)
}

func TestScaleUpNoHelp(t *testing.T) {
	fakeClient := &fake.Clientset{}
	n1 := BuildTestNode("n1", 100, 1000)
	SetNodeReadyState(n1, true, time.Now())

	p1 := BuildTestPod("p1", 80, 0)
	p1.Spec.NodeName = "n1"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		if strings.Contains(fieldstring, "n1") {
			return true, &apiv1.PodList{Items: []apiv1.Pod{*p1}}, nil
		}
		return true, nil, fmt.Errorf("Failed to list: %v", list)
	})

	provider := testprovider.NewTestCloudProvider(func(nodeGroup string, increase int) error {
		t.Fatalf("No expansion is expected")
		return nil
	}, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNode("ng1", n1)
	assert.NotNil(t, provider)

	fakeRecorder := kube_record.NewFakeRecorder(5)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", kube_record.NewFakeRecorder(5), false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)
	clusterState.UpdateNodes([]*apiv1.Node{n1}, time.Now())
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			EstimatorName:  estimator.BinpackingEstimatorName,
			MaxCoresTotal:  config.DefaultMaxClusterCores,
			MaxMemoryTotal: config.DefaultMaxClusterMemory,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}
	p3 := BuildTestPod("p-new", 500, 0)

	result, err := ScaleUp(context, []*apiv1.Pod{p3}, []*apiv1.Node{n1}, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	assert.False(t, result)
	var event string
	select {
	case event = <-fakeRecorder.Events:
	default:
		t.Fatal("No Event recorded, expected NotTriggerScaleUp event")
	}
	assert.Regexp(t, regexp.MustCompile("NotTriggerScaleUp"), event)
}

func TestScaleUpBalanceGroups(t *testing.T) {
	fakeClient := &fake.Clientset{}
	provider := testprovider.NewTestCloudProvider(func(string, int) error {
		return nil
	}, nil)

	type ngInfo struct {
		min, max, size int
	}
	testCfg := map[string]ngInfo{
		"ng1": {min: 1, max: 1, size: 1},
		"ng2": {min: 1, max: 2, size: 1},
		"ng3": {min: 1, max: 5, size: 1},
		"ng4": {min: 1, max: 5, size: 3},
	}
	podMap := make(map[string]*apiv1.Pod, len(testCfg))
	nodes := make([]*apiv1.Node, 0)

	for gid, gconf := range testCfg {
		provider.AddNodeGroup(gid, gconf.min, gconf.max, gconf.size)
		for i := 0; i < gconf.size; i++ {
			nodeName := fmt.Sprintf("%v-node-%v", gid, i)
			node := BuildTestNode(nodeName, 100, 1000)
			SetNodeReadyState(node, true, time.Now())
			nodes = append(nodes, node)

			pod := BuildTestPod(fmt.Sprintf("%v-pod-%v", gid, i), 80, 0)
			pod.Spec.NodeName = nodeName
			podMap[gid] = pod

			provider.AddNode(gid, node)
		}
	}

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		list := action.(core.ListAction)
		fieldstring := list.GetListRestrictions().Fields.String()
		matcher, err := regexp.Compile("ng[0-9]")
		if err != nil {
			return false, &apiv1.PodList{Items: []apiv1.Pod{}}, err
		}
		matches := matcher.FindStringSubmatch(fieldstring)
		if len(matches) != 1 {
			return false, &apiv1.PodList{Items: []apiv1.Pod{}}, fmt.Errorf("parse error")
		}
		return true, &apiv1.PodList{Items: []apiv1.Pod{*(podMap[matches[0]])}}, nil
	})

	fakeRecorder := kube_record.NewFakeRecorder(5)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", kube_record.NewFakeRecorder(5), false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)
	clusterState.UpdateNodes(nodes, time.Now())
	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			EstimatorName:            estimator.BinpackingEstimatorName,
			BalanceSimilarNodeGroups: true,
			MaxCoresTotal:            config.DefaultMaxClusterCores,
			MaxMemoryTotal:           config.DefaultMaxClusterMemory,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}

	pods := make([]*apiv1.Pod, 0)
	for i := 0; i < 2; i++ {
		pods = append(pods, BuildTestPod(fmt.Sprintf("test-pod-%v", i), 80, 0))
	}

	result, typedErr := ScaleUp(context, pods, nodes, []*extensionsv1.DaemonSet{})
	assert.NoError(t, typedErr)
	assert.True(t, result)
	groupMap := make(map[string]cloudprovider.NodeGroup, 3)
	for _, group := range provider.NodeGroups() {
		groupMap[group.Id()] = group
	}

	ng2size, err := groupMap["ng2"].TargetSize()
	assert.NoError(t, err)
	ng3size, err := groupMap["ng3"].TargetSize()
	assert.NoError(t, err)
	assert.Equal(t, 2, ng2size)
	assert.Equal(t, 2, ng3size)
}

func TestScaleUpAutoprovisionedNodeGroup(t *testing.T) {
	createdGroups := make(chan string, 10)
	expandedGroups := make(chan string, 10)

	p1 := BuildTestPod("p1", 80, 0)

	fakeClient := &fake.Clientset{}

	t1 := BuildTestNode("t1", 4000, 1000000)
	SetNodeReadyState(t1, true, time.Time{})
	ti1 := schedulercache.NewNodeInfo()
	ti1.SetNode(t1)

	provider := testprovider.NewTestAutoprovisioningCloudProvider(
		func(nodeGroup string, increase int) error {
			expandedGroups <- fmt.Sprintf("%s-%d", nodeGroup, increase)
			return nil
		}, nil, func(nodeGroup string) error {
			createdGroups <- nodeGroup
			return nil
		}, nil, []string{"T1"}, map[string]*schedulercache.NodeInfo{"T1": ti1})

	fakeRecorder := kube_util.CreateEventRecorder(fakeClient)
	fakeLogRecorder, _ := utils.NewStatusMapRecorder(fakeClient, "kube-system", fakeRecorder, false)
	clusterState := clusterstate.NewClusterStateRegistry(provider, clusterstate.ClusterStateRegistryConfig{}, fakeLogRecorder)

	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			EstimatorName:                    estimator.BinpackingEstimatorName,
			MaxCoresTotal:                    5000 * 64,
			MaxMemoryTotal:                   5000 * 64 * 20,
			NodeAutoprovisioningEnabled:      true,
			MaxAutoprovisionedNodeGroupCount: 10,
		},
		PredicateChecker:     simulator.NewTestPredicateChecker(),
		CloudProvider:        provider,
		ClientSet:            fakeClient,
		Recorder:             fakeRecorder,
		ExpanderStrategy:     random.NewStrategy(),
		ClusterStateRegistry: clusterState,
		LogRecorder:          fakeLogRecorder,
	}

	result, err := ScaleUp(context, []*apiv1.Pod{p1}, []*apiv1.Node{}, []*extensionsv1.DaemonSet{})
	assert.NoError(t, err)
	assert.True(t, result)
	assert.Equal(t, "autoprovisioned-T1", getStringFromChan(createdGroups))
	assert.Equal(t, "autoprovisioned-T1-1", getStringFromChan(expandedGroups))
}

func TestAddAutoprovisionedCandidatesOK(t *testing.T) {
	t1 := BuildTestNode("t1", 4000, 1000000)
	ti1 := schedulercache.NewNodeInfo()
	ti1.SetNode(t1)
	p1 := BuildTestPod("p1", 100, 100)

	n1 := BuildTestNode("ng1-xxx", 4000, 1000000)
	ni1 := schedulercache.NewNodeInfo()
	ni1.SetNode(n1)

	provider := testprovider.NewTestAutoprovisioningCloudProvider(nil, nil,
		nil, nil,
		[]string{"T1"}, map[string]*schedulercache.NodeInfo{"T1": ti1})
	provider.AddNodeGroup("ng1", 1, 5, 3)

	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			MaxAutoprovisionedNodeGroupCount: 1,
		},
		CloudProvider: provider,
	}
	nodeGroups := provider.NodeGroups()
	nodeInfos := map[string]*schedulercache.NodeInfo{
		"ng1": ni1,
	}
	nodeGroups, nodeInfos = addAutoprovisionedCandidates(context, nodeGroups, nodeInfos, []*apiv1.Pod{p1})

	assert.Equal(t, 2, len(nodeGroups))
	assert.Equal(t, 2, len(nodeInfos))
}

func TestAddAutoprovisionedCandidatesToMany(t *testing.T) {
	t1 := BuildTestNode("T1-abc", 4000, 1000000)
	ti1 := schedulercache.NewNodeInfo()
	ti1.SetNode(t1)

	x1 := BuildTestNode("X1-cde", 4000, 1000000)
	xi1 := schedulercache.NewNodeInfo()
	xi1.SetNode(x1)

	p1 := BuildTestPod("p1", 100, 100)

	provider := testprovider.NewTestAutoprovisioningCloudProvider(nil, nil,
		nil, nil,
		[]string{"T1", "X1"},
		map[string]*schedulercache.NodeInfo{"T1": ti1, "X1": xi1})
	provider.AddAutoprovisionedNodeGroup("autoprovisioned-X1", 0, 1000, 0, "X1")

	context := &AutoscalingContext{
		AutoscalingOptions: AutoscalingOptions{
			MaxAutoprovisionedNodeGroupCount: 1,
		},
		CloudProvider: provider,
	}
	nodeGroups := provider.NodeGroups()
	nodeInfos := map[string]*schedulercache.NodeInfo{"X1": xi1}
	nodeGroups, nodeInfos = addAutoprovisionedCandidates(context, nodeGroups, nodeInfos, []*apiv1.Pod{p1})

	assert.Equal(t, 1, len(nodeGroups))
	assert.Equal(t, 1, len(nodeInfos))
}
