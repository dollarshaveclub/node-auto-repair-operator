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

package gce

import (
	"fmt"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/autoscaler/cluster-autoscaler/utils/gpu"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// The 'GCE' cloud provider actually implements both the GCE and GKE providers.
const (
	ProviderNameGCE = "gce"
	ProviderNameGKE = "gke"
)

const (
	maxAutoprovisionedSize = 1000
	minAutoprovisionedSize = 0
)

// Big machines are temporarily commented out.
// TODO(mwielgus): get this list programatically
var autoprovisionedMachineTypes = []string{
	"n1-standard-1",
	"n1-standard-2",
	"n1-standard-4",
	"n1-standard-8",
	"n1-standard-16",
	//"n1-standard-32",
	//"n1-standard-64",
	"n1-highcpu-2",
	"n1-highcpu-4",
	"n1-highcpu-8",
	"n1-highcpu-16",
	//"n1-highcpu-32",
	// "n1-highcpu-64",
	"n1-highmem-2",
	"n1-highmem-4",
	"n1-highmem-8",
	"n1-highmem-16",
	//"n1-highmem-32",
	//"n1-highmem-64",
}

// GceCloudProvider implements CloudProvider interface.
type GceCloudProvider struct {
	gceManager GceManager
	// This resource limiter is used if resource limits are not defined through cloud API.
	resourceLimiterFromFlags *cloudprovider.ResourceLimiter
}

// BuildGceCloudProvider builds CloudProvider implementation for GCE.
func BuildGceCloudProvider(gceManager GceManager, resourceLimiter *cloudprovider.ResourceLimiter) (*GceCloudProvider, error) {
	return &GceCloudProvider{gceManager: gceManager, resourceLimiterFromFlags: resourceLimiter}, nil
}

// Cleanup cleans up all resources before the cloud provider is removed
func (gce *GceCloudProvider) Cleanup() error {
	gce.gceManager.Cleanup()
	return nil
}

// Name returns name of the cloud provider.
func (gce *GceCloudProvider) Name() string {
	// Technically we're both ProviderNameGCE and ProviderNameGKE...
	// Perhaps we should return a different name depending on
	// gce.gceManager.getMode()?
	return ProviderNameGCE
}

// NodeGroups returns all node groups configured for this cloud provider.
func (gce *GceCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	migs := gce.gceManager.getMigs()
	result := make([]cloudprovider.NodeGroup, 0, len(migs))
	for _, mig := range migs {
		result = append(result, mig.config)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (gce *GceCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	ref, err := GceRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	mig, err := gce.gceManager.GetMigForInstance(ref)
	return mig, err
}

// Pricing returns pricing model for this cloud provider or error if not available.
func (gce *GceCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return &GcePriceModel{}, nil
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
func (gce *GceCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return autoprovisionedMachineTypes, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
func (gce *GceCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	nodePoolName := fmt.Sprintf("%s-%s-%d", nodeAutoprovisioningPrefix, machineType, time.Now().Unix())
	zone := gce.gceManager.getLocation()
	taints := make([]apiv1.Taint, 0)

	if gpuRequest, found := extraResources[gpu.ResourceNvidiaGPU]; found {
		gpuType, found := systemLabels[gpu.GPULabel]
		if !found {
			return nil, cloudprovider.ErrIllegalConfiguration
		}
		gpuCount, err := getNormalizedGpuCount(gpuRequest.Value())
		if err != nil {
			return nil, err
		}
		extraResources[gpu.ResourceNvidiaGPU] = *resource.NewQuantity(gpuCount, resource.DecimalSI)
		err = validateGpuConfig(gpuType, gpuCount, zone, machineType)
		if err != nil {
			return nil, err
		}
		nodePoolName = fmt.Sprintf("%s-%s-gpu-%d", nodeAutoprovisioningPrefix, machineType, time.Now().Unix())
		labels[gpu.GPULabel] = gpuType

		taint := apiv1.Taint{
			Effect: apiv1.TaintEffectNoSchedule,
			Key:    "gke-accelerator",
			Value:  gpuType,
		}
		taints = append(taints, taint)
	}

	mig := &Mig{
		autoprovisioned: true,
		exist:           false,
		nodePoolName:    nodePoolName,
		GceRef: GceRef{
			Project: gce.gceManager.getProjectId(),
			Zone:    zone,
			Name:    nodePoolName + "-temporary-mig",
		},
		minSize: minAutoprovisionedSize,
		maxSize: maxAutoprovisionedSize,
		spec: &autoprovisioningSpec{
			machineType:    machineType,
			labels:         labels,
			taints:         taints,
			extraResources: extraResources,
		},
		gceManager: gce.gceManager,
	}
	_, err := gce.gceManager.getTemplates().buildNodeFromAutoprovisioningSpec(mig)
	if err != nil {
		return nil, err
	}
	return mig, nil
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (gce *GceCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	resourceLimiter, err := gce.gceManager.GetResourceLimiter()
	if err != nil {
		return nil, err
	}
	if resourceLimiter != nil {
		return resourceLimiter, nil
	}
	return gce.resourceLimiterFromFlags, nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (gce *GceCloudProvider) Refresh() error {
	return gce.gceManager.Refresh()
}

// GceRef contains s reference to some entity in GCE/GKE world.
type GceRef struct {
	Project string
	Zone    string
	Name    string
}

// GceRefFromProviderId creates InstanceConfig object
// from provider id which must be in format:
// gce://<project-id>/<zone>/<name>
// TODO(piosz): add better check whether the id is correct
func GceRefFromProviderId(id string) (*GceRef, error) {
	splitted := strings.Split(id[6:], "/")
	if len(splitted) != 3 {
		return nil, fmt.Errorf("Wrong id: expected format gce://<project-id>/<zone>/<name>, got %v", id)
	}
	return &GceRef{
		Project: splitted[0],
		Zone:    splitted[1],
		Name:    splitted[2],
	}, nil
}

// Information about what autosprovisioning would like from this mig.
type autoprovisioningSpec struct {
	machineType    string
	labels         map[string]string
	taints         []apiv1.Taint
	extraResources map[string]resource.Quantity
}

// Mig implements NodeGroup interfrace.
type Mig struct {
	GceRef

	gceManager      GceManager
	minSize         int
	maxSize         int
	autoprovisioned bool
	exist           bool
	nodePoolName    string
	spec            *autoprovisioningSpec
}

// MaxSize returns maximum size of the node group.
func (mig *Mig) MaxSize() int {
	return mig.maxSize
}

// MinSize returns minimum size of the node group.
func (mig *Mig) MinSize() int {
	return mig.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kubernetes.
func (mig *Mig) TargetSize() (int, error) {
	if !mig.exist {
		return 0, nil
	}
	size, err := mig.gceManager.GetMigSize(mig)
	return int(size), err
}

// IncreaseSize increases Mig size
func (mig *Mig) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := mig.gceManager.GetMigSize(mig)
	if err != nil {
		return err
	}
	if int(size)+delta > mig.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, mig.MaxSize())
	}
	return mig.gceManager.SetMigSize(mig, size+int64(delta))
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
func (mig *Mig) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease must be negative")
	}
	size, err := mig.gceManager.GetMigSize(mig)
	if err != nil {
		return err
	}
	nodes, err := mig.gceManager.GetMigNodes(mig)
	if err != nil {
		return err
	}
	if int(size)+delta < len(nodes) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%d delta:%d existingNodes: %d",
			size, delta, len(nodes))
	}
	return mig.gceManager.SetMigSize(mig, size+int64(delta))
}

// Belongs returns true if the given node belongs to the NodeGroup.
func (mig *Mig) Belongs(node *apiv1.Node) (bool, error) {
	ref, err := GceRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetMig, err := mig.gceManager.GetMigForInstance(ref)
	if err != nil {
		return false, err
	}
	if targetMig == nil {
		return false, fmt.Errorf("%s doesn't belong to a known mig", node.Name)
	}
	if targetMig.Id() != mig.Id() {
		return false, nil
	}
	return true, nil
}

// DeleteNodes deletes the nodes from the group.
func (mig *Mig) DeleteNodes(nodes []*apiv1.Node) error {
	size, err := mig.gceManager.GetMigSize(mig)
	if err != nil {
		return err
	}
	if int(size) <= mig.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	refs := make([]*GceRef, 0, len(nodes))
	for _, node := range nodes {

		belongs, err := mig.Belongs(node)
		if err != nil {
			return err
		}
		if !belongs {
			return fmt.Errorf("%s belong to a different mig than %s", node.Name, mig.Id())
		}
		gceref, err := GceRefFromProviderId(node.Spec.ProviderID)
		if err != nil {
			return err
		}
		refs = append(refs, gceref)
	}
	return mig.gceManager.DeleteInstances(refs)
}

// Id returns mig url.
func (mig *Mig) Id() string {
	return GenerateMigUrl(mig.Project, mig.Zone, mig.Name)
}

// Debug returns a debug string for the Mig.
func (mig *Mig) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", mig.Id(), mig.MinSize(), mig.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
func (mig *Mig) Nodes() ([]string, error) {
	return mig.gceManager.GetMigNodes(mig)
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one.
func (mig *Mig) Exist() bool {
	return mig.exist
}

// Create creates the node group on the cloud provider side.
func (mig *Mig) Create() error {
	if !mig.exist && mig.autoprovisioned {
		return mig.gceManager.createNodePool(mig)
	}
	return fmt.Errorf("Cannot create non-autoprovisioned node group")
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
func (mig *Mig) Delete() error {
	if mig.exist && mig.autoprovisioned {
		return mig.gceManager.deleteNodePool(mig)
	}
	return fmt.Errorf("Cannot delete non-autoprovisioned node group")
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (mig *Mig) Autoprovisioned() bool {
	return mig.autoprovisioned
}

// TemplateNodeInfo returns a node template for this node group.
func (mig *Mig) TemplateNodeInfo() (*schedulercache.NodeInfo, error) {
	var node *apiv1.Node
	if mig.Exist() {
		template, err := mig.gceManager.getTemplates().getMigTemplate(mig)
		if err != nil {
			return nil, err
		}
		node, err = mig.gceManager.getTemplates().buildNodeFromTemplate(mig, template)
		if err != nil {
			return nil, err
		}
	} else if mig.Autoprovisioned() {
		var err error
		node, err = mig.gceManager.getTemplates().buildNodeFromAutoprovisioningSpec(mig)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unable to get node info for %s/%s/%s", mig.Project, mig.Zone, mig.Name)
	}
	nodeInfo := schedulercache.NewNodeInfo(cloudprovider.BuildKubeProxy(mig.Id()))
	nodeInfo.SetNode(node)
	return nodeInfo, nil
}
