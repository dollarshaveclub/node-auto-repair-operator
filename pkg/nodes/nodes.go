package nodes

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
)

// Node represents metadata about a Kubernetes node.
type Node struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// String is the printable version of a Node.
func (n *Node) String() string {
	return fmt.Sprintf("Node(%s)", n.ID)
}

// NewNodeFromKubeNode creates a new Node using Kubernetes API
// metadata.
func NewNodeFromKubeNode(node *v1.Node) *Node {
	return &Node{
		ID:        node.Status.NodeInfo.MachineID,
		Name:      node.ObjectMeta.Name,
		CreatedAt: node.ObjectMeta.CreationTimestamp.Time,
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
