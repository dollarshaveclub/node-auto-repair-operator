// Code generated by mockery v1.0.0
package mocks

import bolt "github.com/coreos/bbolt"
import mock "github.com/stretchr/testify/mock"
import naro "github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"
import time "time"

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// CreateNode provides a mock function with given fields: node
func (_m *Store) CreateNode(node *naro.Node) error {
	ret := _m.Called(node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*naro.Node) error); ok {
		r0 = rf(node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateNodeEvent provides a mock function with given fields: event
func (_m *Store) CreateNodeEvent(event *naro.NodeEvent) error {
	ret := _m.Called(event)

	var r0 error
	if rf, ok := ret.Get(0).(func(*naro.NodeEvent) error); ok {
		r0 = rf(event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateNodeEventTX provides a mock function with given fields: tx, event
func (_m *Store) CreateNodeEventTX(tx *bolt.Tx, event *naro.NodeEvent) error {
	ret := _m.Called(tx, event)

	var r0 error
	if rf, ok := ret.Get(0).(func(*bolt.Tx, *naro.NodeEvent) error); ok {
		r0 = rf(tx, event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateNodeTX provides a mock function with given fields: tx, node
func (_m *Store) CreateNodeTX(tx *bolt.Tx, node *naro.Node) error {
	ret := _m.Called(tx, node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*bolt.Tx, *naro.Node) error); ok {
		r0 = rf(tx, node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteNode provides a mock function with given fields: node
func (_m *Store) DeleteNode(node *naro.Node) error {
	ret := _m.Called(node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*naro.Node) error); ok {
		r0 = rf(node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteNodeTX provides a mock function with given fields: tx, node
func (_m *Store) DeleteNodeTX(tx *bolt.Tx, node *naro.Node) error {
	ret := _m.Called(tx, node)

	var r0 error
	if rf, ok := ret.Get(0).(func(*bolt.Tx, *naro.Node) error); ok {
		r0 = rf(tx, node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetNode provides a mock function with given fields: nodeID
func (_m *Store) GetNode(nodeID string) (*naro.Node, error) {
	ret := _m.Called(nodeID)

	var r0 *naro.Node
	if rf, ok := ret.Get(0).(func(string) *naro.Node); ok {
		r0 = rf(nodeID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*naro.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(nodeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodeTX provides a mock function with given fields: tx, nodeID
func (_m *Store) GetNodeTX(tx *bolt.Tx, nodeID string) (*naro.Node, error) {
	ret := _m.Called(tx, nodeID)

	var r0 *naro.Node
	if rf, ok := ret.Get(0).(func(*bolt.Tx, string) *naro.Node); ok {
		r0 = rf(tx, nodeID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*naro.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*bolt.Tx, string) error); ok {
		r1 = rf(tx, nodeID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodeTimePeriodSummaries provides a mock function with given fields: start, end
func (_m *Store) GetNodeTimePeriodSummaries(start time.Time, end time.Time) ([]*naro.NodeTimePeriodSummary, error) {
	ret := _m.Called(start, end)

	var r0 []*naro.NodeTimePeriodSummary
	if rf, ok := ret.Get(0).(func(time.Time, time.Time) []*naro.NodeTimePeriodSummary); ok {
		r0 = rf(start, end)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*naro.NodeTimePeriodSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(time.Time, time.Time) error); ok {
		r1 = rf(start, end)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodeTimePeriodSummariesTX provides a mock function with given fields: tx, start, end
func (_m *Store) GetNodeTimePeriodSummariesTX(tx *bolt.Tx, start time.Time, end time.Time) ([]*naro.NodeTimePeriodSummary, error) {
	ret := _m.Called(tx, start, end)

	var r0 []*naro.NodeTimePeriodSummary
	if rf, ok := ret.Get(0).(func(*bolt.Tx, time.Time, time.Time) []*naro.NodeTimePeriodSummary); ok {
		r0 = rf(tx, start, end)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*naro.NodeTimePeriodSummary)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*bolt.Tx, time.Time, time.Time) error); ok {
		r1 = rf(tx, start, end)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WalkNodeEvents provides a mock function with given fields: nodeID, handler
func (_m *Store) WalkNodeEvents(nodeID string, handler func(*naro.NodeEvent) error) error {
	ret := _m.Called(nodeID, handler)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, func(*naro.NodeEvent) error) error); ok {
		r0 = rf(nodeID, handler)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WalkNodeEventsTX provides a mock function with given fields: tx, nodeID, handler
func (_m *Store) WalkNodeEventsTX(tx *bolt.Tx, nodeID string, handler func(*naro.NodeEvent) error) error {
	ret := _m.Called(tx, nodeID, handler)

	var r0 error
	if rf, ok := ret.Get(0).(func(*bolt.Tx, string, func(*naro.NodeEvent) error) error); ok {
		r0 = rf(tx, nodeID, handler)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
