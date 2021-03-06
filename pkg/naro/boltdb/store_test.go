package boltdb_test

import (
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/boltdb"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro/testutil"
	"github.com/stretchr/testify/assert"

	"k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeCRUD(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := boltdb.NewStore(db)
	assert.NoError(t, err)

	node := &naro.Node{
		ID:        "asdf",
		Name:      "A Sdf",
		CreatedAt: time.Now(),
		Source: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "asdf",
			},
		},
	}

	event := &naro.NodeEvent{
		ID:        "1",
		NodeID:    node.ID,
		CreatedAt: time.Now(),
	}
	assert.NoError(t, store.CreateNodeEvent(event))

	t.Run("create", func(t *testing.T) {
		err := db.Update(func(tx *bolt.Tx) error {
			return store.CreateNodeTX(tx, node)
		})
		assert.NoError(t, err)
	})

	t.Run("update", func(t *testing.T) {
		err := db.Update(func(tx *bolt.Tx) error {
			return store.CreateNodeTX(tx, node)
		})
		assert.NoError(t, err)
	})

	t.Run("get", func(t *testing.T) {
		err := db.View(func(tx *bolt.Tx) error {
			n, err := store.GetNodeTX(tx, node.ID)
			assert.NoError(t, err)
			if assert.NotNil(t, n) {
				assert.Equal(t, node.ID, n.ID)
				assert.Equal(t, node.Name, n.Name)
			}
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		err := db.Update(func(tx *bolt.Tx) error {
			assert.NoError(t, store.DeleteNodeTX(tx, node))
			n, err := store.GetNodeTX(tx, node.ID)
			assert.NoError(t, err)
			assert.Nil(t, n)
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("get-node-events", func(t *testing.T) {
		events, err := store.GetNodeEvents(node)
		if assert.NoError(t, err) {
			assert.Len(t, events, 1)
			assert.Equal(t, event.ID, events[0].ID)
		}
	})
}

func TestGetNodeTimePeriodSummaries(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := boltdb.NewStore(db)
	assert.NoError(t, err)

	node := &naro.Node{
		ID:        "asdf",
		Name:      "A Sdf",
		CreatedAt: time.Now(),
	}
	assert.NoError(t, store.CreateNode(node))

	startTime := time.Now()
	endTime := time.Now().Add(time.Hour)

	oldEvent := &naro.NodeEvent{
		ID:        "1",
		NodeID:    node.ID,
		CreatedAt: startTime.Add(-time.Hour),
	}
	assert.NoError(t, store.CreateNodeEvent(oldEvent))

	currentEvent := &naro.NodeEvent{
		ID:        "2",
		NodeID:    node.ID,
		CreatedAt: startTime.Add(time.Minute),
	}
	assert.NoError(t, store.CreateNodeEvent(currentEvent))

	futureEvent := &naro.NodeEvent{
		ID:        "3",
		NodeID:    node.ID,
		CreatedAt: startTime.Add(2 * time.Hour),
	}
	assert.NoError(t, store.CreateNodeEvent(futureEvent))

	t.Run("middle-event", func(t *testing.T) {
		summaries, err := store.GetNodeTimePeriodSummaries(startTime, endTime)
		assert.NoError(t, err)

		if assert.Len(t, summaries, 1) {
			summary := summaries[0]
			assert.Equal(t, startTime, summary.PeriodStart)
			assert.Equal(t, endTime, summary.PeriodEnd)
			assert.Equal(t, node.ID, summary.Node.ID)

			if assert.Len(t, summary.Events, 1) {
				event := summary.Events[0]
				assert.Equal(t, event.ID, currentEvent.ID)
			}
		}
	})

	t.Run("all-events", func(t *testing.T) {
		summaries, err := store.GetNodeTimePeriodSummaries(startTime.Add(-time.Hour), endTime.Add(time.Hour))
		assert.NoError(t, err)

		if assert.Len(t, summaries, 1) {
			summary := summaries[0]
			assert.Equal(t, startTime.Add(-time.Hour), summary.PeriodStart)
			assert.Equal(t, endTime.Add(time.Hour), summary.PeriodEnd)
			assert.Equal(t, node.ID, summary.Node.ID)
			assert.Len(t, summary.Events, 3)
		}
	})
}
