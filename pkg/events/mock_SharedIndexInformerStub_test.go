// Code generated by mockery v1.0.0
package events

import cache "k8s.io/client-go/tools/cache"
import mock "github.com/stretchr/testify/mock"
import time "time"

// MockSharedIndexInformerStub is an autogenerated mock type for the SharedIndexInformerStub type
type MockSharedIndexInformerStub struct {
	mock.Mock
}

// AddEventHandler provides a mock function with given fields: handler
func (_m *MockSharedIndexInformerStub) AddEventHandler(handler cache.ResourceEventHandler) {
	_m.Called(handler)
}

// AddEventHandlerWithResyncPeriod provides a mock function with given fields: handler, resyncPeriod
func (_m *MockSharedIndexInformerStub) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
	_m.Called(handler, resyncPeriod)
}

// AddIndexers provides a mock function with given fields: indexers
func (_m *MockSharedIndexInformerStub) AddIndexers(indexers cache.Indexers) error {
	ret := _m.Called(indexers)

	var r0 error
	if rf, ok := ret.Get(0).(func(cache.Indexers) error); ok {
		r0 = rf(indexers)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetController provides a mock function with given fields:
func (_m *MockSharedIndexInformerStub) GetController() cache.Controller {
	ret := _m.Called()

	var r0 cache.Controller
	if rf, ok := ret.Get(0).(func() cache.Controller); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.Controller)
		}
	}

	return r0
}

// GetIndexer provides a mock function with given fields:
func (_m *MockSharedIndexInformerStub) GetIndexer() cache.Indexer {
	ret := _m.Called()

	var r0 cache.Indexer
	if rf, ok := ret.Get(0).(func() cache.Indexer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.Indexer)
		}
	}

	return r0
}

// GetStore provides a mock function with given fields:
func (_m *MockSharedIndexInformerStub) GetStore() cache.Store {
	ret := _m.Called()

	var r0 cache.Store
	if rf, ok := ret.Get(0).(func() cache.Store); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.Store)
		}
	}

	return r0
}

// HasSynced provides a mock function with given fields:
func (_m *MockSharedIndexInformerStub) HasSynced() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// LastSyncResourceVersion provides a mock function with given fields:
func (_m *MockSharedIndexInformerStub) LastSyncResourceVersion() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Run provides a mock function with given fields: stopCh
func (_m *MockSharedIndexInformerStub) Run(stopCh <-chan struct{}) {
	_m.Called(stopCh)
}
