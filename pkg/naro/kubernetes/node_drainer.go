package kubernetes

import (
	"context"
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	kube_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubernetes "k8s.io/client-go/kubernetes"
	kube_record "k8s.io/client-go/tools/record"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// PodEvictionHeadroom is the extra time we wait to catch situations when the pod is ignoring SIGTERM and
	// is killed with SIGKILL after MaxGracefulTerminationTime
	PodEvictionHeadroom = 30 * time.Second
	// MaxPodEvictionTime is the maximum time CA tries to evict a pod before giving up.
	MaxPodEvictionTime = 2 * time.Minute
	// EvictionRetryTime is the time after CA retries failed pod eviction.
	EvictionRetryTime = 10 * time.Second
	// MaxGracefulTerminationSec is the maximum amount of time to
	// wait for all pods on a node to terminate.
	MaxGracefulTerminationSec = 60 * 30
)

// NodeDrainer drains a Kubernetes node.
type NodeDrainer struct {
	client kubernetes.Interface
}

// NewNodeDrainer instantiates a new NodeDrainer.
func NewNodeDrainer(client kubernetes.Interface) *NodeDrainer {
	return &NodeDrainer{client: client}
}

// Drain performs the drain operation on a node to completion.
func (n *NodeDrainer) Drain(ctx context.Context, node *naro.Node) error {
	fields := fields.Set{"nodeName": node.Name}
	pods, err := n.client.CoreV1().Pods(apiv1.NamespaceAll).
		List(metav1.ListOptions{FieldSelector: fields.String()})
	if err != nil {
		return errors.Wrapf(err, "error listing pods for node")
	}

	// TODO: add recorder
	if err := drainNode(node.Source, pods.Items, n.client, nil, MaxGracefulTerminationSec,
		MaxPodEvictionTime, EvictionRetryTime); err != nil {
		return errors.Wrapf(err, "error draining node")
	}

	return nil
}

// The following code is adapted from
// https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/core/scale_down.go

func drainNode(node *apiv1.Node, pods []apiv1.Pod, client kubernetes.Interface, recorder kube_record.EventRecorder,
	maxGracefulTerminationSec int, maxPodEvictionTime time.Duration, waitBetweenRetries time.Duration) AutoscalerError {

	toEvict := len(pods)
	retryUntil := time.Now().Add(maxPodEvictionTime)
	confirmations := make(chan error, toEvict)
	for _, pod := range pods {
		go func(podToEvict *apiv1.Pod) {
			confirmations <- evictPod(podToEvict, client, recorder, maxGracefulTerminationSec, retryUntil, waitBetweenRetries)
		}(&pod)
	}

	evictionErrs := make([]error, 0)

	for range pods {
		select {
		case err := <-confirmations:
			if err != nil {
				evictionErrs = append(evictionErrs, err)
			} else {
				// Prometheus metrics
				// metrics.RegisterEvictions(1)
			}
		case <-time.After(retryUntil.Sub(time.Now()) + 5*time.Second):
			return NewAutoscalerError(
				ApiCallError, "Failed to drain node %s/%s: timeout when waiting for creating evictions", node.Namespace, node.Name)
		}
	}
	if len(evictionErrs) != 0 {
		return NewAutoscalerError(
			ApiCallError, "Failed to drain node %s/%s, due to following errors: %v", node.Namespace, node.Name, evictionErrs)
	}

	// Evictions created successfully, wait maxGracefulTerminationSec + PodEvictionHeadroom to see if pods really disappeared.
	allGone := true
	for start := time.Now(); time.Now().Sub(start) < time.Duration(maxGracefulTerminationSec)*time.Second+PodEvictionHeadroom; time.Sleep(5 * time.Second) {
		allGone = true
		for _, pod := range pods {
			podreturned, err := client.CoreV1().Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
			if err == nil {
				logrus.Errorf("Not deleted yet %v", podreturned)
				allGone = false
				break
			}
			if !kube_errors.IsNotFound(err) {
				logrus.Errorf("Failed to check pod %s/%s: %v", pod.Namespace, pod.Name, err)
				allGone = false
			}
		}
		if allGone {
			logrus.Infof("All pods removed from %s", node.Name)
			// Let the deferred function know there is no need for cleanup
			return nil
		}
	}
	return NewAutoscalerError(
		TransientError, "Failed to drain node %s/%s: pods remaining after timeout", node.Namespace, node.Name)
}

func evictPod(podToEvict *apiv1.Pod, client kubernetes.Interface, recorder kube_record.EventRecorder,
	maxGracefulTerminationSec int, retryUntil time.Time, waitBetweenRetries time.Duration) error {
	recorder.Eventf(podToEvict, apiv1.EventTypeNormal, "ScaleDown", "deleting pod for node scale down")

	maxTermination := int64(apiv1.DefaultTerminationGracePeriodSeconds)
	if podToEvict.Spec.TerminationGracePeriodSeconds != nil {
		if *podToEvict.Spec.TerminationGracePeriodSeconds < int64(maxGracefulTerminationSec) {
			maxTermination = *podToEvict.Spec.TerminationGracePeriodSeconds
		} else {
			maxTermination = int64(maxGracefulTerminationSec)
		}
	}

	var lastError error
	for first := true; first || time.Now().Before(retryUntil); time.Sleep(waitBetweenRetries) {
		first = false
		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: podToEvict.Namespace,
				Name:      podToEvict.Name,
			},
			DeleteOptions: &metav1.DeleteOptions{
				GracePeriodSeconds: &maxTermination,
			},
		}
		lastError = client.CoreV1().Pods(podToEvict.Namespace).Evict(eviction)
		if lastError == nil || kube_errors.IsNotFound(lastError) {
			return nil
		}
	}
	logrus.Errorf("Failed to evict pod %s, error: %v", podToEvict.Name, lastError)
	recorder.Eventf(podToEvict, apiv1.EventTypeWarning, "ScaleDownFailed", "failed to delete pod for ScaleDown")
	return fmt.Errorf("Failed to evict pod %s/%s within allowed timeout (last error: %v)", podToEvict.Namespace, podToEvict.Name, lastError)
}
