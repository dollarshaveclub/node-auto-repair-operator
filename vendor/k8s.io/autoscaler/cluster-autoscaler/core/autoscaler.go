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
	"time"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config/dynamic"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	kube_util "k8s.io/autoscaler/cluster-autoscaler/utils/kubernetes"
	kube_client "k8s.io/client-go/kubernetes"
	kube_record "k8s.io/client-go/tools/record"
)

// AutoscalerOptions is the whole set of options for configuring an autoscaler
type AutoscalerOptions struct {
	AutoscalingOptions
	dynamic.ConfigFetcherOptions
}

// Autoscaler is the main component of CA which scales up/down node groups according to its configuration
// The configuration can be injected at the creation of an autoscaler
type Autoscaler interface {
	// RunOnce represents an iteration in the control-loop of CA
	RunOnce(currentTime time.Time) errors.AutoscalerError
	// CleanUp represents a clean-up required before the first invocation of RunOnce
	CleanUp()
	// CloudProvider returns the cloud provider associated to this autoscaler
	CloudProvider() cloudprovider.CloudProvider
	// ExitCleanUp is a clean-up performed just before process termination.
	ExitCleanUp()
}

// NewAutoscaler creates an autoscaler of an appropriate type according to the parameters
func NewAutoscaler(opts AutoscalerOptions, predicateChecker *simulator.PredicateChecker, kubeClient kube_client.Interface,
	kubeEventRecorder kube_record.EventRecorder, listerRegistry kube_util.ListerRegistry) (Autoscaler, errors.AutoscalerError) {

	autoscalerBuilder := NewAutoscalerBuilder(opts.AutoscalingOptions, predicateChecker, kubeClient, kubeEventRecorder, listerRegistry)
	if opts.ConfigMapName != "" {
		configFetcher := dynamic.NewConfigFetcher(opts.ConfigFetcherOptions, kubeClient, kubeEventRecorder)
		return NewDynamicAutoscaler(autoscalerBuilder, configFetcher)
	}
	return autoscalerBuilder.Build()
}
