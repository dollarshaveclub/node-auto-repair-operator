package naro

import (
	"context"
	"sync"
	"time"

	bolt "github.com/coreos/bbolt"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type (
	// RepairStatus describes the different states of the node repair
	// state machine.
	RepairStatus string
	// RepairConfigurationName refers to the name of a
	// RepairConfiguration instance.
	RepairConfigurationName string
	// RepairConfigurationVersion refers to the version of a
	// RepairConfiguration.
	RepairConfigurationVersion string
	// RepairStage refers to the current stage in the repair
	// process.
	RepairStage int
)

const (
	RepairStatusHealthy    RepairStatus = "healthy"
	RepairStatusInProgress RepairStatus = "in-progress"
	RepairStatusFailed     RepairStatus = "failed"

	// MaxRepairContinuationDuration is the maximum amount of time
	// in between stages of a repair attempt.
	MaxRepairContinuationDuration = 24 * time.Hour
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
type RepairStrategy func(context.Context, *Node) error

// NodeRepairer can repair a Kubernetes node using a specified repair
// strategy.
type NodeRepairer struct {
	clock     clockwork.Clock
	txCreator TransactionCreator
	store     Store
	drainer   NodeDrainer
	tainter   NodeTainter
}

// NewNodeRepairer instantiates a new NodeRepairer.
func NewNodeRepairer(clock clockwork.Clock, txCreator TransactionCreator, store Store,
	drainer NodeDrainer, tainter NodeTainter) *NodeRepairer {
	return &NodeRepairer{
		clock:     clock,
		txCreator: txCreator,
		store:     store,
		drainer:   drainer,
		tainter:   tainter,
	}
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
	if err := strategy(rctx, node); err != nil {
		return errorHandler(errors.Wrapf(err, "error repairing %s", node))
	}

	// Add NoSchedule taint
	if err := n.tainter.RemoveTaint(node); err != nil {
		return errorHandler(errors.Wrapf(err, "error removing tainting %s", node))
	}

	// TODO: wait until node is healthy

	node.RepairedAt = n.clock.Now()
	node.RepairStatus = RepairStatusHealthy
	if err := n.store.CreateNode(node); err != nil {
		return errors.Wrapf(err, "error updating Node repair status")
	}

	return nil
}

// RepairConfiguration contains the ordered list of repair strategies
// to apply to a node. If the list ever changes, the version should be
// bumped.
type RepairConfiguration struct {
	Name                    RepairConfigurationName
	Version                 RepairConfigurationVersion
	OrderedRepairStrategies []RepairStrategy
}

// RepairController selects the appropriate RepairStrategy for a node
// and applies it.
type RepairController struct {
	sync.Mutex
	clock         clockwork.Clock
	config        *RepairConfiguration
	txCreator     TransactionCreator
	store         Store
	repairer      *NodeRepairer
	nodeIDToMutex map[string]*sync.Mutex
}

// NewRepairController instantiates a new RepairController.
func NewRepairController(clock clockwork.Clock,
	config *RepairConfiguration,
	txCreator TransactionCreator,
	store Store,
	repairer *NodeRepairer) *RepairController {
	return &RepairController{
		clock:         clock,
		config:        config,
		txCreator:     txCreator,
		store:         store,
		repairer:      repairer,
		nodeIDToMutex: make(map[string]*sync.Mutex),
	}
}

func (r *RepairController) selectRepairStrategy(node *Node) (int, error) {
	strategyIdx := 0

	if node.RepairConfigurationName == "" ||
		node.RepairConfigurationName != r.config.Name ||
		node.RepairConfigurationVersion != r.config.Version {
		// Initialize the node to the current repair
		// configuration.
		if err := r.txCreator.Update(func(tx *bolt.Tx) error {
			node.RepairConfigurationName = r.config.Name
			node.RepairConfigurationVersion = r.config.Version
			node.RepairStage = 0

			if err := r.store.CreateNodeTX(tx, node); err != nil {
				return errors.Wrapf(err, "error updating Node repair configuration")
			}

			return nil
		}); err != nil {
			return 0, errors.Wrapf(err, "error writing transaction")
		}
	} else if node.RepairConfigurationName == r.config.Name &&
		node.RepairConfigurationVersion == r.config.Version {
		// If we're continuing to use an existing repair
		// strategy configuration, advance to the next stage
		// if it's been 24 hours since the last repair attempt.
		if node.RepairedAt.Add(MaxRepairContinuationDuration).After(r.clock.Now()) {
			strategyIdx++
		}
	}

	if strategyIdx > len(r.config.OrderedRepairStrategies)-1 {
		// TODO: we probably shouldn't reach this step
		return 0, errors.New("error: no repair strategies left to apply")
	}

	return strategyIdx, nil
}

func (r *RepairController) getLockForNode(node *Node) func() {
	r.Lock()
	if _, ok := r.nodeIDToMutex[node.ID]; !ok {
		r.nodeIDToMutex[node.ID] = &sync.Mutex{}
	}
	defer r.Unlock()

	lock := r.nodeIDToMutex[node.ID]
	lock.Lock()
	return func() {
		lock.Unlock()
	}
}

// HandleAnomaly handles a Node anomaly, selecting a repair strategy
// and applying it.
func (r *RepairController) HandleAnomaly(ctx context.Context, ntps *NodeTimePeriodSummary, anomalyMeta string) error {
	node := ntps.Node
	unlock := r.getLockForNode(node)
	defer unlock()

	strategyIdx, err := r.selectRepairStrategy(node)
	if err != nil {
		return errors.Wrapf(err, "error selecting repair strategy for %s", node)
	}

	// Write the selected stage.
	if err := r.txCreator.Update(func(tx *bolt.Tx) error {
		node.RepairStage = RepairStage(strategyIdx)

		if err := r.store.CreateNodeTX(tx, node); err != nil {
			return errors.Wrapf(err, "error updating Node repair stage")
		}

		return nil
	}); err != nil {
		return errors.Wrapf(err, "error writing transaction")
	}

	logrus.Infof("repairing %s with repair configuration %s:%s with strategy %d",
		node, r.config.Name, r.config.Version, strategyIdx)

	if err := r.repairer.RepairNode(ctx, node, r.config.OrderedRepairStrategies[strategyIdx]); err != nil {
		return errors.Wrapf(err, "error repairing %s", node)
	}

	logrus.Infof("finished repairing %s")

	return nil
}
