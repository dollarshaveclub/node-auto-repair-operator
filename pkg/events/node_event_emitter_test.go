package events_test

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/Sirupsen/logrus"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/events"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestNodeEventEmitterHandleEvent(t *testing.T) {
	nodeEventHandler := &events.MockKubeNodeEventHandler{}
	informer := &events.MockSharedIndexInformerStub{}
	defer mock.AssertExpectationsForObjects(t, nodeEventHandler, informer)

	event := &v1.Event{
		InvolvedObject: v1.ObjectReference{
			Kind: "Node",
		},
		Message: "Node has issues",
	}

	var eventHandler cache.ResourceEventHandler

	nodeEventHandler.On("HandleKubeNodeEvent", event).Run(func(args mock.Arguments) {
		e := args.Get(0).(*v1.Event)
		assert.Equal(t, event.InvolvedObject.Kind, e.InvolvedObject.Kind)
		assert.Equal(t, event.Message, e.Message)
	}).Return(nil)
	informer.On("AddEventHandlerWithResyncPeriod", mock.Anything, time.Minute).Run(func(args mock.Arguments) {
		eventHandler = args.Get(0).(cache.ResourceEventHandler)
	}).Return()
	informer.On("Run", mock.Anything).Return()

	emitter := events.NewKubeNodeEventEmitter(informer, time.Minute)
	emitter.AddHandler(nodeEventHandler)
	emitter.Start()

	eventHandler.OnAdd(event)
}

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}
