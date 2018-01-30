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

package drain

import (
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	client "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubelet/types"
)

const (
	// PodDeletionTimeout - time after which a pod to be deleted is not included in the list of pods for drain.
	PodDeletionTimeout = 12 * time.Minute
)

const (
	// PodSafeToEvictKey - annotation that ignores constraints to evict a pod like not being replicated, being on
	// kube-system namespace or having a local storage.
	PodSafeToEvictKey = "cluster-autoscaler.kubernetes.io/safe-to-evict"
)

// GetPodsForDeletionOnNodeDrain returns pods that should be deleted on node drain as well as some extra information
// about possibly problematic pods (unreplicated and daemonsets).
func GetPodsForDeletionOnNodeDrain(
	podList []*apiv1.Pod,
	pdbs []*policyv1.PodDisruptionBudget,
	deleteAll bool,
	skipNodesWithSystemPods bool,
	skipNodesWithLocalStorage bool,
	checkReferences bool, // Setting this to true requires client to be not-null.
	client client.Interface,
	minReplica int32,
	currentTime time.Time) ([]*apiv1.Pod, error) {

	pods := []*apiv1.Pod{}
	// filter kube-system PDBs to avoid doing it for every kube-system pod
	kubeSystemPDBs := make([]*policyv1.PodDisruptionBudget, 0)
	for _, pdb := range pdbs {
		if pdb.Namespace == "kube-system" {
			kubeSystemPDBs = append(kubeSystemPDBs, pdb)
		}
	}

	for _, pod := range podList {
		if IsMirrorPod(pod) {
			continue
		}

		// Possibly skip a pod under deletion but only if it was being deleted for long enough
		// to avoid a situation when we delete the empty node immediately after the pod was marked for
		// deletion without respecting any graceful termination.
		if pod.DeletionTimestamp != nil && pod.DeletionTimestamp.Time.Before(currentTime.Add(-1*PodDeletionTimeout)) {
			// pod is being deleted for long enough - no need to care about it.
			continue
		}

		daemonsetPod := false
		replicated := false
		safeToEvict := hasSaveToEvictAnnotation(pod)

		controllerRef := ControllerRef(pod)
		refKind := ""
		if controllerRef != nil {
			refKind = controllerRef.Kind
		}

		// For now, owner controller must be in the same namespace as the pod
		// so OwnerReference doesn't have its own Namespace field
		controllerNamespace := pod.Namespace

		if refKind == "ReplicationController" {
			if checkReferences {
				rc, err := client.CoreV1().ReplicationControllers(controllerNamespace).Get(controllerRef.Name, metav1.GetOptions{})
				// Assume a reason for an error is because the RC is either
				// gone/missing or that the rc has too few replicas configured.
				// TODO: replace the minReplica check with pod disruption budget.
				if err == nil && rc != nil {
					if rc.Spec.Replicas != nil && *rc.Spec.Replicas < minReplica {
						return []*apiv1.Pod{}, fmt.Errorf("replication controller for %s/%s has too few replicas spec: %d min: %d",
							pod.Namespace, pod.Name, rc.Spec.Replicas, minReplica)
					}
					replicated = true

				} else {
					return []*apiv1.Pod{}, fmt.Errorf("replication controller for %s/%s is not available, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		} else if refKind == "DaemonSet" {
			if checkReferences {
				ds, err := client.ExtensionsV1beta1().DaemonSets(controllerNamespace).Get(controllerRef.Name, metav1.GetOptions{})

				// Assume the only reason for an error is because the DaemonSet is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && ds != nil {
					// Otherwise, treat daemonset-managed pods as unmanaged since
					// DaemonSet Controller currently ignores the unschedulable bit.
					// FIXME(mml): Add link to the issue concerning a proper way to drain
					// daemonset pods, probably using taints.
					daemonsetPod = true
				} else {
					return []*apiv1.Pod{}, fmt.Errorf("daemonset for %s/%s is not present, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				daemonsetPod = true
			}
		} else if refKind == "Job" {
			if checkReferences {
				job, err := client.BatchV1().Jobs(controllerNamespace).Get(controllerRef.Name, metav1.GetOptions{})

				// Assume the only reason for an error is because the Job is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && job != nil {
					replicated = true
				} else {
					return []*apiv1.Pod{}, fmt.Errorf("job for %s/%s is not available: err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		} else if refKind == "ReplicaSet" {
			if checkReferences {
				rs, err := client.ExtensionsV1beta1().ReplicaSets(controllerNamespace).Get(controllerRef.Name, metav1.GetOptions{})

				// Assume the only reason for an error is because the RS is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && rs != nil {
					if rs.Spec.Replicas != nil && *rs.Spec.Replicas < minReplica {
						return []*apiv1.Pod{}, fmt.Errorf("replication controller for %s/%s has too few replicas spec: %d min: %d",
							pod.Namespace, pod.Name, rs.Spec.Replicas, minReplica)
					}
					replicated = true
				} else {
					return []*apiv1.Pod{}, fmt.Errorf("replication controller for %s/%s is not available, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		} else if refKind == "StatefulSet" {
			if checkReferences {
				ss, err := client.AppsV1beta1().StatefulSets(controllerNamespace).Get(controllerRef.Name, metav1.GetOptions{})

				// Assume the only reason for an error is because the StatefulSet is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && ss != nil {
					replicated = true
				} else {
					return []*apiv1.Pod{}, fmt.Errorf("statefulset for %s/%s is not available: err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		}
		if daemonsetPod {
			continue
		}
		if !deleteAll && !safeToEvict {
			if !replicated {
				return []*apiv1.Pod{}, fmt.Errorf("%s/%s is not replicated", pod.Namespace, pod.Name)
			}
			if pod.Namespace == "kube-system" && skipNodesWithSystemPods {
				hasPDB, err := checkKubeSystemPDBs(pod, kubeSystemPDBs)
				if err != nil {
					return []*apiv1.Pod{}, fmt.Errorf("error matching pods to pdbs: %v", err)
				}
				if !hasPDB {
					return []*apiv1.Pod{}, fmt.Errorf("non-daemonset, non-mirrored, non-pdb-assigned kube-system pod present: %s", pod.Name)
				}
			}
			if HasLocalStorage(pod) && skipNodesWithLocalStorage {
				return []*apiv1.Pod{}, fmt.Errorf("pod with local storage present: %s", pod.Name)
			}
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// ControllerRef returns the OwnerReference to pod's controller.
func ControllerRef(pod *apiv1.Pod) *metav1.OwnerReference {
	return metav1.GetControllerOf(pod)
}

// IsMirrorPod checks whether the pod is a mirror pod.
func IsMirrorPod(pod *apiv1.Pod) bool {
	_, found := pod.ObjectMeta.Annotations[types.ConfigMirrorAnnotationKey]
	return found
}

// HasLocalStorage returns true if pod has any local storage.
func HasLocalStorage(pod *apiv1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if isLocalVolume(&volume) {
			return true
		}
	}
	return false
}

func isLocalVolume(volume *apiv1.Volume) bool {
	return volume.HostPath != nil || volume.EmptyDir != nil
}

// This only checks if a matching PDB exist and therefore if it makes sense to attempt drain simulation,
// as we check for allowed-disruptions later anyway (for all pods with PDB, not just in kube-system)
func checkKubeSystemPDBs(pod *apiv1.Pod, pdbs []*policyv1.PodDisruptionBudget) (bool, error) {
	for _, pdb := range pdbs {
		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			return false, err
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			return true, nil
		}
	}

	return false, nil
}

// This checks if pod has PodSafeToEvictKey annotation
func hasSaveToEvictAnnotation(pod *apiv1.Pod) bool {
	return pod.GetAnnotations()[PodSafeToEvictKey] == "true"
}
