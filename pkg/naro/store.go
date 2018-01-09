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
	CreateNodeTX(tx *bolt.Tx, node *Node) error
	DeleteNode(node *Node) error
	DeleteNodeTX(tx *bolt.Tx, node *Node) error
	GetNode(nodeID string) (*Node, error)
	GetNodeTX(tx *bolt.Tx, nodeID string) (*Node, error)
	GetNodeTimePeriodSummaries(start, end time.Time) ([]*NodeTimePeriodSummary, error)
	GetNodeTimePeriodSummariesTX(tx *bolt.Tx, start, end time.Time) ([]*NodeTimePeriodSummary, error)
	WalkNodeEvents(nodeID string, handler func(*NodeEvent) error) error
	WalkNodeEventsTX(tx *bolt.Tx, nodeID string, handler func(*NodeEvent) error) error
}
