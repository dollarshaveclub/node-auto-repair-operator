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

package main

import (
	"time"

	"k8s.io/autoscaler/vertical-pod-autoscaler/updater/eviction"
	"k8s.io/autoscaler/vertical-pod-autoscaler/updater/priority"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	vpa_types "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/poc.autoscaling.k8s.io/v1alpha1"
	vpa_clientset "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	vpa_lister "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/listers/poc.autoscaling.k8s.io/v1alpha1"
	vpa_api_util "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/utils/vpa"
	kube_client "k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
)

// Updater performs updates on pods if recommended by Vertical Pod Autoscaler
type Updater interface {
	// RunOnce represents single iteration in the main-loop of Updater
	RunOnce()
}

type updater struct {
	vpaLister        vpa_lister.VerticalPodAutoscalerLister
	podLister        v1lister.PodLister
	evictionFactrory eviction.PodsEvictionRestrictionFactory
}

// NewUpdater creates Updater with given configuration
func NewUpdater(kubeClient kube_client.Interface, vpaClient *vpa_clientset.Clientset, cacheTTl time.Duration, minReplicasForEvicition int, evictionToleranceFraction float64) Updater {
	return &updater{
		vpaLister:        vpa_api_util.NewAllVpasLister(vpaClient, make(chan struct{})),
		podLister:        newPodLister(kubeClient),
		evictionFactrory: eviction.NewPodsEvictionRestrictionFactory(kubeClient, minReplicasForEvicition, evictionToleranceFraction),
	}
}

// RunOnce represents single iteration in the main-loop of Updater
func (u *updater) RunOnce() {
	vpaList, err := u.vpaLister.List(labels.Everything())
	if err != nil {
		glog.Fatalf("failed get VPA list: %v", err)
	}

	if len(vpaList) == 0 {
		glog.Warningf("no VPA objects to process")
		return
	}

	for _, vpa := range vpaList {
		glog.V(2).Infof("processing VPA object targeting %v", vpa.Spec.Selector)
		selector, err := metav1.LabelSelectorAsSelector(vpa.Spec.Selector)
		if err != nil {
			glog.Errorf("error processing VPA object: failed to create pod selector: %v", err)
			continue
		}

		podsList, err := u.podLister.Pods(vpa.Namespace).List(selector)
		if err != nil {
			glog.Errorf("failed get pods list for namespace %v and selector %v: %v", vpa.Namespace, selector, err)
			continue
		}

		livePods := filterDeletedPods(podsList)
		if len(livePods) == 0 {
			glog.Warningf("no live pods matching selector %v", selector)
			continue
		}

		evictionLimiter := u.evictionFactrory.NewPodsEvictionRestriction(livePods)
		podsForUpdate := u.getPodsForUpdate(filterNonEvictablePods(livePods, evictionLimiter), vpa)

		for _, pod := range podsForUpdate {
			if !evictionLimiter.CanEvict(pod) {
				continue
			}
			glog.V(2).Infof("evicting pod %v", pod.Name)
			evictErr := evictionLimiter.Evict(pod)
			if evictErr != nil {
				glog.Warningf("evicting pod %v failed: %v", pod.Name, evictErr)
			}
		}
	}
}

// getPodsForUpdate returns list of pods that should be updated ordered by update priority
func (u *updater) getPodsForUpdate(pods []*apiv1.Pod, vpa *vpa_types.VerticalPodAutoscaler) []*apiv1.Pod {
	priorityCalculator := priority.NewUpdatePriorityCalculator(&vpa.Spec.ResourcePolicy, nil)
	recommendation := vpa.Status.Recommendation

	for _, pod := range pods {
		priorityCalculator.AddPod(pod, &recommendation)
	}

	return priorityCalculator.GetSortedPods()
}

func filterNonEvictablePods(pods []*apiv1.Pod, evictionRestriciton eviction.PodsEvictionRestriction) []*apiv1.Pod {
	result := make([]*apiv1.Pod, 0)
	for _, pod := range pods {
		if evictionRestriciton.CanEvict(pod) {
			result = append(result, pod)
		}
	}
	return result
}

func filterDeletedPods(pods []*apiv1.Pod) []*apiv1.Pod {
	result := make([]*apiv1.Pod, 0)
	for _, pod := range pods {
		if pod.DeletionTimestamp == nil {
			result = append(result, pod)
		}
	}
	return result
}

func newPodLister(kubeClient kube_client.Interface) v1lister.PodLister {
	selector := fields.ParseSelectorOrDie("spec.nodeName!=" + "" + ",status.phase!=" +
		string(apiv1.PodSucceeded) + ",status.phase!=" + string(apiv1.PodFailed))
	podListWatch := cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", apiv1.NamespaceAll, selector)
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podLister := v1lister.NewPodLister(store)
	podReflector := cache.NewReflector(podListWatch, &apiv1.Pod{}, store, time.Hour)
	stopCh := make(chan struct{})
	go podReflector.Run(stopCh)

	return podLister
}
