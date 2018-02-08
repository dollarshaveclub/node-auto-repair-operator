package naro

import (
	"fmt"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/typed/core/v1"
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
	Source          *corev1.Event
}

// String is the printable version of a NodeEvent.
func (n *NodeEvent) String() string {
	return fmt.Sprintf("NodeEvent(%s)", n.ID)
}

// NewNodeEventFromKubeEvent creates a new NodeEvent from a Kubernetes
// event.
func NewNodeEventFromKubeEvent(node *Node, event *corev1.Event) *NodeEvent {
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

// KubeNodeEventHandler is an interface for a type that can ingest a
// node event.
type KubeNodeEventHandler interface {
	HandleKubeNodeEvent(*corev1.Event) error
}

// KubeNodeEventController is a type that watches for Kubernetes
// events. When an event is detected, the event is persisted with a
// Store.
type KubeNodeEventController struct {
	db         *bolt.DB
	nodeClient v1.NodeInterface
	store      Store
}

// String is the string representation of a controller.
func (k *KubeNodeEventController) String() string {
	return "KubeNodeEventController"
}

// NewKubeNodeEventController creates a new KubeNodeEventController.
func NewKubeNodeEventController(db *bolt.DB, nodeClient v1.NodeInterface,
	store Store) *KubeNodeEventController {
	return &KubeNodeEventController{
		db:         db,
		nodeClient: nodeClient,
		store:      store,
	}
}

// HandleKubeNodeEvent handles a Kubernetes event.
func (k *KubeNodeEventController) HandleKubeNodeEvent(e *corev1.Event) error {
	// TODO: potentially cache this call. Fetching nodes for each
	// event might be expensive.
	kubeNode, err := k.nodeClient.Get(e.InvolvedObject.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "error fetching Kubernetes node")
	}

	if err := k.db.Update(func(tx *bolt.Tx) error {
		node, err := k.store.GetNode(NewNodeFromKubeNode(kubeNode).ID)
		if err != nil {
			return errors.Wrapf(err, "error fetching Node")
		}
		if node == nil {
			node = NewNodeFromKubeNode(kubeNode)
			if err := k.store.CreateNodeTX(tx, node); err != nil {
				return errors.Wrapf(err, "error creating Node")
			}
		}

		event := NewNodeEventFromKubeEvent(node, e)
		if err := k.store.CreateNodeEventTX(tx, event); err != nil {
			return errors.Wrapf(err, "error creating NodeEvent")
		}

		logrus.Infof("KubeNodeEventController: processed event %s for node %s", event, node)

		return nil
	}); err != nil {
		return errors.Wrapf(err, "error running DB update")
	}

	return nil

}
