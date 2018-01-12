package naro

import (
	"context"
	"time"

	bolt "github.com/coreos/bbolt"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

const (
	RepairStatusHealthy    RepairStatus = "healthy"
	RepairStatusInProgress RepairStatus = "in-progress"
	RepairStatusFailed     RepairStatus = "failed"
)

type RepairStatus string

type NodeDrainer interface {
	Drain(context.Context, *Node) error
}

type RepairStrategy interface {
	RepairNode(context.Context, *Node) error
}

type NodeRepairer struct {
	txCreator  TransactionCreator
	store      Store
	strategies []RepairStrategy
	drainer    NodeDrainer
}

func (n *NodeRepairer) RepairNode(ctx context.Context, node *Node, strategy RepairStrategy) error {
	// Mark node as being repaired
	if err := n.txCreator.Update(func(tx *bolt.Tx) error {
		if node.RepairStatus != "" {
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

	// Archive NodeEvents. This is necessary so that the node
	// isn't rescheduled to be repaired.
	if err := n.store.DeleteNodeEvents(node); err != nil {
		return errors.Wrapf(err, "error deleting NodeEvents for %s", node)
	}

	node.RepairStatus = RepairStatusHealthy
	if err := n.store.CreateNode(node); err != nil {
		return errors.Wrapf(err, "error updating Node repair status")
	}

	return nil
}
