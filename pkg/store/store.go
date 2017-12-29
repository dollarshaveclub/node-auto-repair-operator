package store

import (
	"encoding/json"
	"fmt"

	bolt "github.com/coreos/bbolt"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/pkg/errors"
)

var (
	nodeBucketName   = []byte("nodes")
	eventsBucketName = []byte("events")
)

func nodeEventBucket(nodeID string) []byte {
	return []byte(fmt.Sprintf("events:%s", nodeID))
}

// Store manages persisting node data to disk using boltdb.
type Store struct {
	db *bolt.DB
}

// NewStore instantiates a new Store.
func NewStore(db *bolt.DB) (*Store, error) {
	n := &Store{db: db}
	if err := n.initializeBuckets(); err != nil {
		return nil, errors.Wrapf(err, "error initializing buckets for node store")
	}
	return n, nil
}

func (n *Store) initializeBuckets() error {
	if err := n.db.Update(func(tx *bolt.Tx) error {
		// Create all initial buckets.
		buckets := [][]byte{nodeBucketName, eventsBucketName}
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return errors.Wrapf(err, "error creating bucket %s", bucket)
			}
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error initializing buckets")
	}

	return nil
}

// CreateNode persists a Node. This method is used for creates and
// updates.
func (n *Store) CreateNode(node *nodes.Node) error {
	if err := n.db.Update(func(tx *bolt.Tx) error {
		if err := n.CreateNodeTX(tx, node); err != nil {
			return errors.Wrapf(err, "error creating node")
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error updating database")
	}

	return nil
}

// CreateNodeTX persists a Node. This method is used for creates and
// updates.
func (n *Store) CreateNodeTX(tx *bolt.Tx, node *nodes.Node) error {
	if err := node.Validate(); err != nil {
		return errors.Wrapf(err, "error validating node")
	}

	buf, err := json.Marshal(node)
	if err != nil {
		return errors.Wrapf(err, "error encoding node")
	}

	nodeBucket := tx.Bucket(nodeBucketName)
	if err := nodeBucket.Put(node.Key(), buf); err != nil {
		return errors.Wrapf(err, "error writing node")
	}

	return nil
}

// GetNode fetches a node by its ID. `nil` is returned if the node
// isn't found.
func (n *Store) GetNode(nodeID string) (*nodes.Node, error) {
	var node *nodes.Node
	if err := n.db.View(func(tx *bolt.Tx) error {
		n, err := n.GetNodeTX(tx, nodeID)
		if err != nil {
			return errors.Wrapf(err, "error fetching node")
		}
		node = n
		return nil
	}); err != nil {
		return nil, errors.Wrapf(err, "error opening transaction")
	}

	return node, nil
}

// GetNodeTX fetches a node by its ID. `nil` is returned if the node
// isn't found.
func (n *Store) GetNodeTX(tx *bolt.Tx, nodeID string) (*nodes.Node, error) {
	if nodeID == "" {
		return nil, errors.New("error: invalid ID provided")
	}

	nodeBucket := tx.Bucket(nodeBucketName)
	buf := nodeBucket.Get(nodes.NodeKey(nodeID))
	if buf == nil {
		return nil, nil
	}

	var node nodes.Node
	if err := json.Unmarshal(buf, &node); err != nil {
		return nil, errors.Wrapf(err, "error unmarshaling into node buffer")
	}

	return &node, nil
}

// DeleteNode deletes a node.
func (n *Store) DeleteNode(node *nodes.Node) error {
	if err := n.db.Update(func(tx *bolt.Tx) error {
		if err := n.DeleteNodeTX(tx, node); err != nil {
			return errors.Wrapf(err, "error deleting node")
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error updating database")
	}

	return nil
}

// DeleteNodeTX deletes a node.
func (n *Store) DeleteNodeTX(tx *bolt.Tx, node *nodes.Node) error {
	if err := node.Validate(); err != nil {
		return errors.Wrapf(err, "error validating node")
	}

	nodeBucket := tx.Bucket(nodeBucketName)
	if err := nodeBucket.Delete(node.Key()); err != nil {
		return errors.Wrapf(err, "error deleting node")
	}

	return nil
}

// CreateNodeEvent persists a NodeEvent.
func (n *Store) CreateNodeEvent(event *nodes.NodeEvent) error {
	if err := n.db.Update(func(tx *bolt.Tx) error {
		if err := n.CreateNodeEventTX(tx, event); err != nil {
			return errors.Wrapf(err, "error creating node event")
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error updating database")
	}

	return nil
}

// CreateNodeEventTX persists a NodeEvent.
func (n *Store) CreateNodeEventTX(tx *bolt.Tx, event *nodes.NodeEvent) error {
	if err := event.Validate(); err != nil {
		return errors.Wrapf(err, "error validating node event")
	}

	eventsBucket := tx.Bucket(eventsBucketName)
	id, err := eventsBucket.NextSequence()
	if err != nil {
		return errors.Wrapf(err, "error generating node event ID")
	}
	event.ID = id

	buf, err := json.Marshal(event)
	if err != nil {
		return errors.Wrapf(err, "error encoding node event")
	}

	eventBucket, err := eventsBucket.CreateBucketIfNotExists(nodeEventBucket(event.NodeID))
	if err != nil {
		return errors.Wrapf(err, "error creating node events bucket")
	}
	if err := eventBucket.Put(event.Key(), buf); err != nil {
		return errors.Wrapf(err, "error writing node event")
	}

	return nil
}

// WalkNodeEvents walks through all events for a Node, calling a
// handler for each individual event.
func (n *Store) WalkNodeEvents(nodeID string, handler func(*nodes.NodeEvent) error) error {
	if err := n.db.View(func(tx *bolt.Tx) error {
		if err := n.WalkNodeEventsTX(tx, nodeID, handler); err != nil {
			return errors.Wrapf(err, "error walking through node events")
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error viewing database")
	}

	return nil
}

// WalkNodeEventsTX walks through all events for a Node, calling a
// handler for each individual event.
func (n *Store) WalkNodeEventsTX(tx *bolt.Tx, nodeID string, handler func(*nodes.NodeEvent) error) error {
	// If the event bucket doesn't exist, then no events have been
	// created.
	eventsBucket := tx.Bucket(eventsBucketName)
	eventBucket := eventsBucket.Bucket(nodeEventBucket(nodeID))
	if eventBucket == nil {
		return nil
	}

	if err := eventBucket.ForEach(func(_ []byte, v []byte) error {
		var event nodes.NodeEvent
		if err := json.Unmarshal(v, &event); err != nil {
			return errors.Wrapf(err, "error unmarshaling node event")
		}
		if err := handler(&event); err != nil {
			return errors.Wrapf(err, "error handling node event")
		}
		return nil
	}); err != nil {
		return errors.Wrapf(err, "error iterating through node event bucket")
	}

	return nil
}