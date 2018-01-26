package naro

import (
	"time"

	bolt "github.com/coreos/bbolt"
)

// Store is an interface for a persistent store backend.
type Store interface {
	CreateNode(node *Node) error
	CreateNodeEvent(event *NodeEvent) error
	CreateNodeEventTX(tx *bolt.Tx, event *NodeEvent) error
	GetNodeEvents(*Node) ([]*NodeEvent, error)
	GetNodeEventsTX(*bolt.Tx, *Node) ([]*NodeEvent, error)
	CreateNodeTX(tx *bolt.Tx, node *Node) error
	DeleteNode(node *Node) error
	DeleteNodeTX(tx *bolt.Tx, node *Node) error
	GetNode(nodeID string) (*Node, error)
	GetNodeTX(tx *bolt.Tx, nodeID string) (*Node, error)
	GetNodeTimePeriodSummaries(start, end time.Time) ([]*NodeTimePeriodSummary, error)
	GetNodeTimePeriodSummariesTX(tx *bolt.Tx, start, end time.Time) ([]*NodeTimePeriodSummary, error)
	DeleteNodeEvents(*Node) error
	DeleteNodeEventsTX(*bolt.Tx, *Node) error
}

// TransactionCreator is a type that can create boltdb transactions.
type TransactionCreator interface {
	View(fn func(*bolt.Tx) error) error
	Update(fn func(*bolt.Tx) error) error
}
