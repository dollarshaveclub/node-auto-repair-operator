package kubernetes

import (
	"fmt"
	"time"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// RepairTaint is a taint used to make the node unschedulable.
	RepairTaint = "RepairingWithNodeAutoRepairOperator"
)

// NodeRepairTainter describes a type that can add and remove repair
// taints from a Kubernetes node. The repair taint prevents additional
// pods from being scheduled onto the node.
type NodeRepairTainter struct {
	client kubernetes.Interface
}

// NewNodeRepairTainter creates a new NewNodeRepairTainter.
func NewNodeRepairTainter(client kubernetes.Interface) *NodeRepairTainter {
	return &NodeRepairTainter{client: client}
}

// Taint applies the NoSchedule taint to the node.
func (n *NodeRepairTainter) Taint(node *naro.Node) error {
	return MarkToBeRepaired(node.Source, n.client)
}

// RemoveTaint removes the NoSchedule from the node.
func (n *NodeRepairTainter) RemoveTaint(node *naro.Node) error {
	_, err := CleanToBeRepaired(node.Source, n.client)
	return err
}

// The code that follows is adapted from
// https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/utils/deletetaint/delete.go

// MarkToBeRepaired sets a taint that makes the node unschedulable.
func MarkToBeRepaired(node *apiv1.Node, client kubernetes.Interface) error {
	// Get the newest version of the node.
	freshNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || freshNode == nil {
		return fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}

	added, err := addRepairTaint(freshNode)
	if added == false {
		return err
	}
	_, err = client.CoreV1().Nodes().Update(freshNode)
	if err != nil {
		return err
	}
	return nil
}

func addRepairTaint(node *apiv1.Node) (bool, error) {
	for _, taint := range node.Spec.Taints {
		if taint.Key == RepairTaint {
			return false, nil
		}
	}
	node.Spec.Taints = append(node.Spec.Taints, apiv1.Taint{
		Key:    RepairTaint,
		Value:  fmt.Sprint(time.Now().Unix()),
		Effect: apiv1.TaintEffectNoSchedule,
	})
	return true, nil
}

// CleanToBeRepaired cleans ToBeRepaired taint.
func CleanToBeRepaired(node *apiv1.Node, client kubernetes.Interface) (bool, error) {
	freshNode, err := client.CoreV1().Nodes().Get(node.Name, metav1.GetOptions{})
	if err != nil || freshNode == nil {
		return false, fmt.Errorf("failed to get node %v: %v", node.Name, err)
	}
	newTaints := make([]apiv1.Taint, 0)
	for _, taint := range freshNode.Spec.Taints {
		if taint.Key == RepairTaint {
		} else {
			newTaints = append(newTaints, taint)
		}
	}

	if len(newTaints) != len(freshNode.Spec.Taints) {
		freshNode.Spec.Taints = newTaints
		_, err := client.CoreV1().Nodes().Update(freshNode)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
