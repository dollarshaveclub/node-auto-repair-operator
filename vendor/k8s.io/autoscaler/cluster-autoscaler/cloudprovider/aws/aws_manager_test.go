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

package aws

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

func TestBuildGenericLabels(t *testing.T) {
	labels := buildGenericLabels(&asgTemplate{
		InstanceType: &instanceType{
			InstanceType: "c4.large",
			VCPU:         2,
			MemoryMb:     3840,
		},
		Region: "us-east-1",
	}, "sillyname")
	assert.Equal(t, "us-east-1", labels[kubeletapis.LabelZoneRegion])
	assert.Equal(t, "sillyname", labels[kubeletapis.LabelHostname])
	assert.Equal(t, "c4.large", labels[kubeletapis.LabelInstanceType])
	assert.Equal(t, cloudprovider.DefaultArch, labels[kubeletapis.LabelArch])
	assert.Equal(t, cloudprovider.DefaultOS, labels[kubeletapis.LabelOS])
}

func TestExtractLabelsFromAsg(t *testing.T) {
	tags := []*autoscaling.TagDescription{
		{
			Key:   aws.String("k8s.io/cluster-autoscaler/node-template/label/foo"),
			Value: aws.String("bar"),
		},
		{
			Key:   aws.String("bar"),
			Value: aws.String("baz"),
		},
	}

	labels := extractLabelsFromAsg(tags)

	assert.Equal(t, 1, len(labels))
	assert.Equal(t, "bar", labels["foo"])
}

func TestExtractTaintsFromAsg(t *testing.T) {
	tags := []*autoscaling.TagDescription{
		{
			Key:   aws.String("k8s.io/cluster-autoscaler/node-template/taint/dedicated"),
			Value: aws.String("foo:NoSchedule"),
		},
		{
			Key:   aws.String("bar"),
			Value: aws.String("baz"),
		},
	}

	expectedTaints := []apiv1.Taint{
		{
			Key:    "dedicated",
			Value:  "foo",
			Effect: apiv1.TaintEffectNoSchedule,
		},
	}

	taints := extractTaintsFromAsg(tags)
	assert.Equal(t, 1, len(taints))
	assert.Equal(t, makeTaintSet(expectedTaints), makeTaintSet(taints))
}

func makeTaintSet(taints []apiv1.Taint) map[apiv1.Taint]bool {
	set := make(map[apiv1.Taint]bool)
	for _, taint := range taints {
		set[taint] = true
	}
	return set
}
func TestBuildAsg(t *testing.T) {
	do := cloudprovider.NodeGroupDiscoveryOptions{}
	m, err := createAWSManagerInternal(nil, do, &testService)
	assert.NoError(t, err)

	asg, err := m.buildAsgFromSpec("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, asg.MinSize(), 1)
	assert.Equal(t, asg.MaxSize(), 5)
	assert.Equal(t, asg.Id(), "test-asg")
	assert.Equal(t, asg.Name, "test-asg")
	assert.Equal(t, asg.Debug(), "test-asg (1:5)")

	_, err = m.buildAsgFromSpec("a")
	assert.Error(t, err)
	_, err = m.buildAsgFromSpec("a:b:c")
	assert.Error(t, err)
	_, err = m.buildAsgFromSpec("1:")
	assert.Error(t, err)
	_, err = m.buildAsgFromSpec("1:2:")
	assert.Error(t, err)
}

func validateAsg(t *testing.T, asg *Asg, name string, minSize int, maxSize int) {
	assert.Equal(t, name, asg.Name)
	assert.Equal(t, minSize, asg.minSize)
	assert.Equal(t, maxSize, asg.maxSize)
}

func TestFetchExplicitAsgs(t *testing.T) {
	min, max, groupname := 1, 10, "coolasg"

	s := &AutoScalingMock{}
	s.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(groupname)},
		MaxRecords:            aws.Int64(1),
	}).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			{AutoScalingGroupName: aws.String(groupname)},
		},
	})

	s.On("DescribeAutoScalingGroupsPages",
		&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{groupname}),
			MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool)
		fn(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{AutoScalingGroupName: aws.String(groupname)},
			}}, false)
	}).Return(nil)

	do := cloudprovider.NodeGroupDiscoveryOptions{
		// Register the same node group twice with different max nodes.
		// The intention is to test that the asgs.Register method will update
		// the node group instead of registering it twice.
		NodeGroupSpecs: []string{
			fmt.Sprintf("%d:%d:%s", min, max-1, groupname),
			fmt.Sprintf("%d:%d:%s", min, max, groupname),
		},
	}
	// fetchExplicitASGs is called at manager creation time.
	m, err := createAWSManagerInternal(nil, do, &autoScalingWrapper{s})
	assert.NoError(t, err)

	asgs := m.asgCache.get()
	assert.Equal(t, 1, len(asgs))
	validateAsg(t, asgs[0].config, groupname, min, max)
}

func TestFetchAutoAsgs(t *testing.T) {
	min, max := 1, 10
	groupname, tags := "coolasg", []string{"tag", "anothertag"}

	s := &AutoScalingMock{}
	// Lookup groups associated with tags
	s.On("DescribeTagsPages",
		&autoscaling.DescribeTagsInput{
			Filters: []*autoscaling.Filter{
				{Name: aws.String("key"), Values: aws.StringSlice([]string{tags[0]})},
				{Name: aws.String("key"), Values: aws.StringSlice([]string{tags[1]})},
			},
			MaxRecords: aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeTagsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeTagsOutput, bool) bool)
		fn(&autoscaling.DescribeTagsOutput{
			Tags: []*autoscaling.TagDescription{
				{ResourceId: aws.String(groupname)},
				{ResourceId: aws.String(groupname)},
			}}, false)
	}).Return(nil).Once()

	// Describe the group to register it, then again to generate the instance
	// cache.
	s.On("DescribeAutoScalingGroupsPages",
		&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{groupname}),
			MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool)
		fn(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{{
				AutoScalingGroupName: aws.String(groupname),
				MinSize:              aws.Int64(int64(min)),
				MaxSize:              aws.Int64(int64(max)),
			}}}, false)
	}).Return(nil).Twice()

	do := cloudprovider.NodeGroupDiscoveryOptions{
		NodeGroupAutoDiscoverySpecs: []string{fmt.Sprintf("asg:tag=%s", strings.Join(tags, ","))},
	}

	// fetchAutoASGs is called at manager creation time, via forceRefresh
	m, err := createAWSManagerInternal(nil, do, &autoScalingWrapper{s})
	assert.NoError(t, err)

	asgs := m.asgCache.get()
	assert.Equal(t, 1, len(asgs))
	validateAsg(t, asgs[0].config, groupname, min, max)

	// Simulate the previously discovered ASG disappearing
	s.On("DescribeTagsPages",
		&autoscaling.DescribeTagsInput{
			Filters: []*autoscaling.Filter{
				{Name: aws.String("key"), Values: aws.StringSlice([]string{tags[0]})},
				{Name: aws.String("key"), Values: aws.StringSlice([]string{tags[1]})},
			},
			MaxRecords: aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeTagsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeTagsOutput, bool) bool)
		fn(&autoscaling.DescribeTagsOutput{Tags: []*autoscaling.TagDescription{}}, false)
	}).Return(nil).Once()

	err = m.fetchAutoAsgs()
	assert.NoError(t, err)
	assert.Empty(t, m.asgCache.get())
}
