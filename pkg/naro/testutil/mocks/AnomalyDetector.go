// Code generated by mockery v1.0.0
package mocks

import mock "github.com/stretchr/testify/mock"
import naro "github.com/dollarshaveclub/node-auto-repair-operator/pkg/naro"

// AnomalyDetector is an autogenerated mock type for the AnomalyDetector type
type AnomalyDetector struct {
	mock.Mock
}

// IsAnomalous provides a mock function with given fields: ns
func (_m *AnomalyDetector) IsAnomalous(ns *naro.NodeTimePeriodSummary) (bool, string, error) {
	ret := _m.Called(ns)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*naro.NodeTimePeriodSummary) bool); ok {
		r0 = rf(ns)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 string
	if rf, ok := ret.Get(1).(func(*naro.NodeTimePeriodSummary) string); ok {
		r1 = rf(ns)
	} else {
		r1 = ret.Get(1).(string)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*naro.NodeTimePeriodSummary) error); ok {
		r2 = rf(ns)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// String provides a mock function with given fields:
func (_m *AnomalyDetector) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Train provides a mock function with given fields: summaries
func (_m *AnomalyDetector) Train(summaries []*naro.NodeTimePeriodSummary) error {
	ret := _m.Called(summaries)

	var r0 error
	if rf, ok := ret.Get(0).(func([]*naro.NodeTimePeriodSummary) error); ok {
		r0 = rf(summaries)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
