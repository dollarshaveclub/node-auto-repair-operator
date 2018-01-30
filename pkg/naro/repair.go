package naro

import (
	"context"
	"time"

	"code.cloudfoundry.org/clock"

	bolt "github.com/coreos/bbolt"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

// RepairStatus describes the different states of the node repair
// state machine.
type RepairStatus string

const (
	RepairStatusHealthy    RepairStatus = "healthy"
	RepairStatusInProgress RepairStatus = "in-progress"
	RepairStatusFailed     RepairStatus = "failed"
)

// NodeTainter describes a taint that prevents pods from being
// scheduled onto a node.
type NodeTainter interface {
	Taint(*Node) error
	RemoveTaint(*Node) error
}

// NodeDrainer describes an interface that can drain pods from a
// Kubernetes node.
type NodeDrainer interface {
	Drain(context.Context, *Node) error
}

// RepairStrategy describes a node repair strategy. Example: a
// strategy can restart a node.
type RepairStrategy interface {
	RepairNode(context.Context, *Node) error
}

// NodeRepairer can repair a Kubernetes node using a specified repair
// strategy.
type NodeRepairer struct {
	clock      clock.Clock
	txCreator  TransactionCreator
	store      Store
	strategies []RepairStrategy
	drainer    NodeDrainer
	tainter    NodeTainter
}

// RepairNode conducts the node repair process on a Kubernetes node
// using the input strategy.
func (n *NodeRepairer) RepairNode(ctx context.Context, node *Node, strategy RepairStrategy) error {
	// Mark node as being repaired
	if err := n.txCreator.Update(func(tx *bolt.Tx) error {
		if node.RepairStatus != RepairStatusHealthy {
			return errors.Errorf("error: can't repair %s since it's in state: %s", node, node.RepairStatus)
		}

		node.RepairStatus = RepairStatusInProgress
		if err := n.store.CreateNodeTX(tx, node); err != nil {
			return errors.Wrapf(err, "error updating Node repair status")
		}

		return nil
	}); err != nil {
		return errors.Wrapf(err, "error writing transaction")
	}

	// This is triggered if any of the repair operations fail
	errorHandler := func(rootErr error) error {
		node.RepairStatus = RepairStatusFailed
		if err := n.store.CreateNode(node); err != nil {
			return multierror.Append(
				rootErr, errors.Wrapf(err, "error updating Node repair status"))
		}
		return nil
	}

	// Add NoSchedule taint
	if err := n.tainter.Taint(node); err != nil {
		return errorHandler(errors.Wrapf(err, "error tainting %s", node))
	}

	// Drain node
	dctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := n.drainer.Drain(dctx, node); err != nil {
		return errorHandler(errors.Wrapf(err, "error draining %s", node))
	}

	// Repair node
	rctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := strategy.RepairNode(rctx, node); err != nil {
		return errorHandler(errors.Wrapf(err, "error repairing %s", node))
	}

	// Add NoSchedule taint
	if err := n.tainter.RemoveTaint(node); err != nil {
		return errorHandler(errors.Wrapf(err, "error removing tainting %s", node))
	}

	node.RepairedAt = n.clock.Now()
	node.RepairStatus = RepairStatusHealthy
	if err := n.store.CreateNode(node); err != nil {
		return errors.Wrapf(err, "error updating Node repair status")
	}

	return nil
}
