package kubernetes

import (
	"sync"
	"time"

	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	nodeEventKind = "Node"
)

// KubeNodeEventEmitter emits Kubernetes node events to handlers.
type KubeNodeEventEmitter struct {
	wg       *sync.WaitGroup
	informer cache.SharedIndexInformer
	stopChan chan struct{}
	handlers []naro.KubeNodeEventHandler
}

// NewKubeNodeEventEmitter instantiates a new KubeNodeEventEmitter.
func NewKubeNodeEventEmitter(informer cache.SharedIndexInformer, syncPeriod time.Duration,
	handlers []naro.KubeNodeEventHandler) *KubeNodeEventEmitter {
	n := &KubeNodeEventEmitter{
		wg:       &sync.WaitGroup{},
		informer: informer,
		stopChan: make(chan struct{}),
		handlers: handlers,
	}

	n.informer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: n.handleEvent,
	}, syncPeriod)

	return n
}

// Start begins the event emission process.
func (n *KubeNodeEventEmitter) Start() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.informer.Run(n.stopChan)
	}()
}

// handleEvent distributes an event to all subscribed handlers.
func (n *KubeNodeEventEmitter) handleEvent(obj interface{}) {
	event, ok := obj.(*v1.Event)
	if !ok {
		logrus.Debugf("KubeNodeEventEmitter: received non-event type")
		return
	}
	if event.InvolvedObject.Kind != nodeEventKind {
		logrus.Debugf("KubeNodeEventEmitter: received non-node event")
		return
	}

	for _, handler := range n.handlers {
		logrus.Debugf("KubeNodeEventEmitter: distributing node event to handler: %s", handler)

		if err := handler.HandleKubeNodeEvent(event); err != nil {
			logrus.WithError(err).Errorf("KubeNodeEventEmitter: error handling node event")
		}
	}
}

// Stop stops the KubeNodeEventEmitter from emitting events.
func (n *KubeNodeEventEmitter) Stop() {
	close(n.stopChan)
	n.wg.Wait()
}

// SharedIndexInformerStub is a stub so that mockery generates a mock
// for this external type.
type SharedIndexInformerStub interface {
	cache.SharedIndexInformer
}
