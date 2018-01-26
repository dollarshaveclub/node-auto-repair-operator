package naro_test

import (
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/boltdb"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/testutil"
	"github.com/stretchr/testify/assert"

	"k8s.io/client-go/kubernetes/fake"
)

func TestKubeNodeEventControllerHandleKubeNodeEvent(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := boltdb.NewStore(db)
	assert.NoError(t, err)

	node := testutil.FakeKubeNode(t)
	kubeClient := fake.NewSimpleClientset(node)
	nodeClient := kubeClient.CoreV1().Nodes()

	controller := naro.NewKubeNodeEventController(db, nodeClient, store)

	event := testutil.FakeKubeNodeEvent(t)

	if assert.NoError(t, controller.HandleKubeNodeEvent(event)) {
		n, err := store.GetNode(node.Status.NodeInfo.SystemUUID)
		assert.NoError(t, err)
		assert.NotNil(t, n)

		events, err := store.GetNodeEvents(&naro.Node{ID: node.Status.NodeInfo.SystemUUID})
		if assert.NoError(t, err) {
			assert.Len(t, events, 1)
		}
	}
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}
