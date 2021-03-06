package naro

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
)

// Node represents metadata about a Kubernetes node.
type Node struct {
	ID                         string
	Name                       string
	CreatedAt                  time.Time
	RepairedAt                 time.Time
	Source                     *v1.Node
	RepairStatus               RepairStatus
	RepairConfigurationName    RepairConfigurationName
	RepairConfigurationVersion RepairConfigurationVersion
	RepairStage                RepairStage
}

// String is the printable version of a Node.
func (n *Node) String() string {
	return fmt.Sprintf("Node(%s)", n.ID)
}

// NewNodeFromKubeNode creates a new Node using Kubernetes API
// metadata.
func NewNodeFromKubeNode(node *v1.Node) *Node {
	return &Node{
		// We can't use MachineID because it isn't unique
		// between instances that share an AMI.
		ID:           node.Status.NodeInfo.SystemUUID,
		Name:         node.ObjectMeta.Name,
		CreatedAt:    node.ObjectMeta.CreationTimestamp.Time,
		Source:       node,
		RepairStatus: RepairStatusHealthy,
	}
}

// NodeKey returns the database key for a Node ID.
func NodeKey(nodeID string) []byte {
	return []byte(fmt.Sprintf("node:%s", nodeID))
}

// Key is the boltdb key for this node.
func (n *Node) Key() []byte {
	return NodeKey(n.ID)
}

// Validate ensures that a Node is persistable by the database.
func (n *Node) Validate() error {
	if n.ID == "" {
		return errors.New("error: Node is missing ID")
	}
	if n.CreatedAt.IsZero() {
		return errors.New("error: Node is missing CreatedAt")
	}
	return nil
}

// A NodeTimePeriodSummary contains all node metadata for a Node
// within a time period.
type NodeTimePeriodSummary struct {
	Node        *Node
	Events      []*NodeEvent
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// RemoveOlderRepairedEvents removes events from the summary that
// occurred before the Node's RepairedAt value.
func (n *NodeTimePeriodSummary) RemoveOlderRepairedEvents() {
	var recentEvents []*NodeEvent
	for _, e := range n.Events {
		if e.CreatedAt.After(n.Node.RepairedAt) {
			recentEvents = append(recentEvents, e)
		}
	}
	n.Events = recentEvents
}
