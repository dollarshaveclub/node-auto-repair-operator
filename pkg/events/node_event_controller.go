package events

import (
	"github.com/Sirupsen/logrus"
	"github.com/coreos/bbolt"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/store"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
)

// KubeNodeEventController is a type that watches for Kubernetes
// events. When an event is detected, the event is persisted with a
// store.Store.
type KubeNodeEventController struct {
	db         *bolt.DB
	nodeClient v1.NodeInterface
	store      *store.Store
}

// String is the string representation of a controller.
func (k *KubeNodeEventController) String() string {
	return "KubeNodeEventController"
}

// NewKubeNodeEventController creates a new KubeNodeEventController.
func NewKubeNodeEventController(db *bolt.DB, nodeClient v1.NodeInterface,
	store *store.Store) *KubeNodeEventController {
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
		// Create/update node
		node := nodes.NewNodeFromKubeNode(kubeNode)
		if err := k.store.CreateNodeTX(tx, node); err != nil {
			return errors.Wrapf(err, "error creating Node")
		}

		event := nodes.NewNodeEventFromKubeEvent(node, e)
		if err := k.store.CreateNodeEventTX(tx, event); err != nil {
			return errors.Wrapf(err, "error creating NodeEvent")
		}

		logrus.Infof("processed event %s for node %s", event, node)

		return nil
	}); err != nil {
		return errors.Wrapf(err, "error running DB update")
	}

	return nil
}