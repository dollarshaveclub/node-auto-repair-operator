package events_test

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/events"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/store"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"

	"k8s.io/client-go/kubernetes/fake"
)

func TestKubeNodeEventControllerHandleKubeNodeEvent(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := store.NewStore(db)
	assert.NoError(t, err)

	node := testutil.FakeKubeNode(t)
	kubeClient := fake.NewSimpleClientset(node)
	nodeClient := kubeClient.CoreV1().Nodes()

	controller := events.NewKubeNodeEventController(db, nodeClient, store)

	event := testutil.FakeKubeNodeEvent(t)

	if assert.NoError(t, controller.HandleKubeNodeEvent(event)) {
		n, err := store.GetNode(node.Status.NodeInfo.SystemUUID)
		assert.NoError(t, err)
		assert.NotNil(t, n)

		var events []*nodes.NodeEvent
		eventHandler := func(e *nodes.NodeEvent) error {
			events = append(events, e)
			return nil
		}
		assert.NoError(t, store.WalkNodeEvents(node.Status.NodeInfo.SystemUUID, eventHandler))
		assert.Len(t, events, 1)
	}
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}
