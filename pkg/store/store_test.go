package store_test

import (
	"testing"
	"time"

	bolt "github.com/coreos/bbolt"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/store"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNodeCRUD(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := store.NewStore(db)
	assert.NoError(t, err)

	node := &nodes.Node{
		ID:        "asdf",
		Name:      "A Sdf",
		CreatedAt: time.Now(),
	}

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
}

func TestNodeEventCreateAndWalk(t *testing.T) {
	db, cleaner := testutil.DB(t)
	defer cleaner()

	store, err := store.NewStore(db)
	assert.NoError(t, err)

	event := &nodes.NodeEvent{
		ID:        "fdsa",
		NodeID:    "asdf",
		CreatedAt: time.Now(),
	}

	t.Run("create", func(t *testing.T) {
		err := db.Update(func(tx *bolt.Tx) error {
			assert.NoError(t, store.CreateNodeEventTX(tx, event))
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("walk", func(t *testing.T) {
		var events []*nodes.NodeEvent
		err := db.Update(func(tx *bolt.Tx) error {
			err := store.WalkNodeEventsTX(tx, event.NodeID, func(e *nodes.NodeEvent) error {
				events = append(events, e)
				return nil
			})
			assert.NoError(t, err)
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(events), "%v", events)
	})
}
