package naro

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
)

// NodeEvent describes a node event emitted by Kubernetes.
type NodeEvent struct {
	NodeID          string
	ID              string
	CreatedAt       time.Time
	InvolvedObject  string
	Kind            string
	Reason          string
	Type            string
	SourceComponent string
	Source          *v1.Event
}

// String is the printable version of a NodeEvent.
func (n *NodeEvent) String() string {
	return fmt.Sprintf("NodeEvent(%s)", n.ID)
}

// NewNodeEventFromKubeEvent creates a new NodeEvent from a Kubernetes
// event.
func NewNodeEventFromKubeEvent(node *Node, event *v1.Event) *NodeEvent {
	return &NodeEvent{
		ID:              string(event.UID),
		NodeID:          node.ID,
		CreatedAt:       event.LastTimestamp.Time,
		InvolvedObject:  event.InvolvedObject.Kind,
		Kind:            event.Kind,
		Reason:          event.Reason,
		Type:            event.Type,
		SourceComponent: event.Source.Component,
		Source:          event,
	}
}

// Key is the boltdb key for this event. This key is sort-able by time
// since it's prefixed with an RFC 3339 timestamp.
func (n *NodeEvent) Key() []byte {
	k := fmt.Sprintf("event:%s:%s", n.CreatedAt.UTC().Format(time.RFC3339), n.ID)
	return []byte(k)
}

// Validate ensures that a NodeEvent is persistable by the database.
func (n *NodeEvent) Validate() error {
	if n.ID == "" {
		return errors.New("error: NodeEvent is missing ID")
	}
	if n.NodeID == "" {
		return errors.New("error: NodeEvent is missing NodeID")
	}
	if n.CreatedAt.IsZero() {
		return errors.New("error: NodeEvent is missing CreatedAt")
	}
	return nil
}
