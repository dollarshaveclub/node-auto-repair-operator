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

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
)

type AutoScalingMock struct {
	mock.Mock
}

func (a *AutoScalingMock) DescribeAutoScalingGroups(i *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := a.Called(i)
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), nil
}

func (a *AutoScalingMock) DescribeAutoScalingGroupsPages(i *autoscaling.DescribeAutoScalingGroupsInput, fn func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	args := a.Called(i, fn)
	return args.Error(0)
}

func (a *AutoScalingMock) DescribeLaunchConfigurations(i *autoscaling.DescribeLaunchConfigurationsInput) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
	args := a.Called(i)
	return args.Get(0).(*autoscaling.DescribeLaunchConfigurationsOutput), nil
}

func (a *AutoScalingMock) DescribeTagsPages(i *autoscaling.DescribeTagsInput, fn func(*autoscaling.DescribeTagsOutput, bool) bool) error {
	args := a.Called(i, fn)
	return args.Error(0)
}

func (a *AutoScalingMock) SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	args := a.Called(input)
	return args.Get(0).(*autoscaling.SetDesiredCapacityOutput), nil
}

func (a *AutoScalingMock) TerminateInstanceInAutoScalingGroup(input *autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	args := a.Called(input)
	return args.Get(0).(*autoscaling.TerminateInstanceInAutoScalingGroupOutput), nil
}

var testService = autoScalingWrapper{&AutoScalingMock{}}

var testAwsManager = &AwsManager{
	asgCache: &asgCache{
		registeredAsgs: make([]*asgInformation, 0),
		instanceToAsg:  make(map[AwsRef]*Asg),
		interrupt:      make(chan struct{}),
		service:        testService,
	},
	explicitlyConfigured: make(map[AwsRef]bool),
	service:              testService,
}

func newTestAwsManagerWithService(service autoScaling) *AwsManager {
	wrapper := autoScalingWrapper{service}
	return &AwsManager{
		service: wrapper,
		asgCache: &asgCache{
			registeredAsgs: make([]*asgInformation, 0),
			instanceToAsg:  make(map[AwsRef]*Asg),
			interrupt:      make(chan struct{}),
			service:        wrapper,
		},
		explicitlyConfigured: make(map[AwsRef]bool),
	}
}

func newTestAwsManagerWithAsgs(t *testing.T, service autoScaling, specs []string) *AwsManager {
	m := newTestAwsManagerWithService(service)
	for _, spec := range specs {
		asg, err := m.buildAsgFromSpec(spec)
		if err != nil {
			t.Fatalf("bad ASG spec %v: %v", spec, err)
		}
		m.RegisterAsg(asg)
	}
	return m
}

func testDescribeAutoScalingGroupsOutput(desiredCap int64, instanceIds ...string) *autoscaling.DescribeAutoScalingGroupsOutput {
	return testNamedDescribeAutoScalingGroupsOutput("UNUSED", desiredCap, instanceIds...)
}

func testNamedDescribeAutoScalingGroupsOutput(groupName string, desiredCap int64, instanceIds ...string) *autoscaling.DescribeAutoScalingGroupsOutput {
	instances := []*autoscaling.Instance{}
	for _, id := range instanceIds {
		instances = append(instances, &autoscaling.Instance{
			InstanceId: aws.String(id),
		})
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []*autoscaling.Group{
			{
				AutoScalingGroupName: aws.String(groupName),
				DesiredCapacity:      aws.Int64(desiredCap),
				Instances:            instances,
			},
		},
	}
}

func testProvider(t *testing.T, m *AwsManager) *awsCloudProvider {
	resourceLimiter := cloudprovider.NewResourceLimiter(
		map[string]int64{cloudprovider.ResourceNameCores: 1, cloudprovider.ResourceNameMemory: 10000000},
		map[string]int64{cloudprovider.ResourceNameCores: 10, cloudprovider.ResourceNameMemory: 100000000})

	provider, err := BuildAwsCloudProvider(m, resourceLimiter)
	assert.NoError(t, err)
	return provider.(*awsCloudProvider)
}

func TestBuildAwsCloudProvider(t *testing.T) {
	resourceLimiter := cloudprovider.NewResourceLimiter(
		map[string]int64{cloudprovider.ResourceNameCores: 1, cloudprovider.ResourceNameMemory: 10000000},
		map[string]int64{cloudprovider.ResourceNameCores: 10, cloudprovider.ResourceNameMemory: 100000000})

	_, err := BuildAwsCloudProvider(testAwsManager, resourceLimiter)
	assert.NoError(t, err)
}

func TestName(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	assert.Equal(t, provider.Name(), ProviderName)
}

func TestNodeGroups(t *testing.T) {
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, testService, []string{"1:5:test-asg"}))
	assert.Equal(t, len(provider.NodeGroups()), 1)
}

func TestNodeGroupForNode(t *testing.T) {
	node := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	service := &AutoScalingMock{}
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, service, []string{"1:5:test-asg"}))
	asgs := provider.asgs()

	service.On("DescribeAutoScalingGroupsPages",
		&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
			MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool)
		fn(testNamedDescribeAutoScalingGroupsOutput("test-asg", 1, "test-instance-id"), false)
	}).Return(nil)

	group, err := provider.NodeGroupForNode(node)

	assert.NoError(t, err)
	assert.Equal(t, group.Id(), "test-asg")
	assert.Equal(t, group.MinSize(), 1)
	assert.Equal(t, group.MaxSize(), 5)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroupsPages", 1)

	// test node in cluster that is not in a group managed by cluster autoscaler
	nodeNotInGroup := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id-not-in-group",
		},
	}

	group, err = provider.NodeGroupForNode(nodeNotInGroup)

	assert.NoError(t, err)
	assert.Nil(t, group)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroupsPages", 2)
}

func TestAwsRefFromProviderId(t *testing.T) {
	_, err := AwsRefFromProviderId("aws123")
	assert.Error(t, err)
	_, err = AwsRefFromProviderId("aws://test-az/test-instance-id")
	assert.Error(t, err)

	awsRef, err := AwsRefFromProviderId("aws:///us-east-1a/i-260942b3")
	assert.NoError(t, err)
	assert.Equal(t, awsRef, &AwsRef{Name: "i-260942b3"})
}

func TestTargetSize(t *testing.T) {
	service := &AutoScalingMock{}
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, service, []string{"1:5:test-asg"}))
	asgs := provider.asgs()

	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
		MaxRecords:            aws.Int64(1),
	}).Return(testDescribeAutoScalingGroupsOutput(2, "test-instance-id", "second-test-instance-id"))

	targetSize, err := asgs[0].TargetSize()
	assert.Equal(t, targetSize, 2)
	assert.NoError(t, err)

	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroups", 1)
}

func TestIncreaseSize(t *testing.T) {
	service := &AutoScalingMock{}
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, service, []string{"1:5:test-asg"}))
	asgs := provider.asgs()

	service.On("SetDesiredCapacity", &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(asgs[0].Name),
		DesiredCapacity:      aws.Int64(3),
		HonorCooldown:        aws.Bool(false),
	}).Return(&autoscaling.SetDesiredCapacityOutput{})

	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
		MaxRecords:            aws.Int64(1),
	}).Return(testDescribeAutoScalingGroupsOutput(2, "test-instance-id", "second-test-instance-id"))

	err := asgs[0].IncreaseSize(1)
	assert.NoError(t, err)
	service.AssertNumberOfCalls(t, "SetDesiredCapacity", 1)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroups", 1)
}

func TestBelongs(t *testing.T) {
	service := &AutoScalingMock{}
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, service, []string{"1:5:test-asg"}))
	asgs := provider.asgs()

	service.On("DescribeAutoScalingGroupsPages",
		&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
			MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool)
		fn(testNamedDescribeAutoScalingGroupsOutput("test-asg", 1, "test-instance-id"), false)
	}).Return(nil)

	invalidNode := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/invalid-instance-id",
		},
	}
	_, err := asgs[0].Belongs(invalidNode)
	assert.Error(t, err)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroupsPages", 1)

	validNode := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	belongs, err := asgs[0].Belongs(validNode)
	assert.Equal(t, belongs, true)
	assert.NoError(t, err)
	// As "test-instance-id" is already known to be managed by test-asg since
	// the first `Belongs` call, No additional DescribAutoScalingGroupsPages
	// call is made
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroupsPages", 1)
}

func TestDeleteNodes(t *testing.T) {
	service := &AutoScalingMock{}
	provider := testProvider(t, newTestAwsManagerWithAsgs(t, service, []string{"1:5:test-asg"}))
	asgs := provider.asgs()

	service.On("TerminateInstanceInAutoScalingGroup", &autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     aws.String("test-instance-id"),
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	}).Return(&autoscaling.TerminateInstanceInAutoScalingGroupOutput{
		Activity: &autoscaling.Activity{Description: aws.String("Deleted instance")},
	})

	// Look up the current number of instances...
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
		MaxRecords:            aws.Int64(1),
	}).Return(testDescribeAutoScalingGroupsOutput(2, "test-instance-id", "second-test-instance-id"))

	// Refresh the instance to ASG cache...
	service.On("DescribeAutoScalingGroupsPages",
		&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgs[0].Name}),
			MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
		},
		mock.AnythingOfType("func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool"),
	).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool)
		fn(testNamedDescribeAutoScalingGroupsOutput("test-asg", 2, "test-instance-id", "second-test-instance-id"), false)
	}).Return(nil)

	node := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	err := asgs[0].DeleteNodes([]*apiv1.Node{node})
	assert.NoError(t, err)
	service.AssertNumberOfCalls(t, "TerminateInstanceInAutoScalingGroup", 1)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroups", 1)
	service.AssertNumberOfCalls(t, "DescribeAutoScalingGroupsPages", 1)
}

func TestGetResourceLimiter(t *testing.T) {
	service := &AutoScalingMock{}
	m := newTestAwsManagerWithService(service)

	provider := testProvider(t, m)
	_, err := provider.GetResourceLimiter()
	assert.NoError(t, err)
}

func TestCleanup(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	err := provider.Cleanup()
	assert.NoError(t, err)
}
