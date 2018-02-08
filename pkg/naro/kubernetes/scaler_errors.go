package kubernetes

import (
	"fmt"
)

// AutoscalerErrorType describes a high-level category of a given error
type AutoscalerErrorType string

// AutoscalerError contains information about Autoscaler errors
type AutoscalerError interface {
	// Error implements golang error interface
	Error() string

	// Type returns the typ of AutoscalerError
	Type() AutoscalerErrorType

	// AddPrefix adds a prefix to error message.
	// Returns the error it's called for convenient inline use.
	// Example:
	// if err := DoSomething(myObject); err != nil {
	//	return err.AddPrefix("can't do something with %v: ", myObject)
	// }
	AddPrefix(msg string, args ...interface{}) AutoscalerError
}

type autoscalerErrorImpl struct {
	errorType AutoscalerErrorType
	msg       string
}

const (
	// CloudProviderError is an error related to underlying infrastructure
	CloudProviderError AutoscalerErrorType = "cloudProviderError"
	// ApiCallError is an error related to communication with k8s API server
	ApiCallError AutoscalerErrorType = "apiCallError"
	// InternalError is an error inside Cluster Autoscaler
	InternalError AutoscalerErrorType = "internalError"
	// TransientError is an error that causes us to skip a single loop, but
	// does not require any additional action.
	TransientError AutoscalerErrorType = "transientError"
)

// NewAutoscalerError returns new autoscaler error with a message constructed from format string
func NewAutoscalerError(errorType AutoscalerErrorType, msg string, args ...interface{}) AutoscalerError {
	return autoscalerErrorImpl{
		errorType: errorType,
		msg:       fmt.Sprintf(msg, args...),
	}
}

// ToAutoscalerError converts an error to AutoscalerError with given type,
// unless it already is an AutoscalerError (in which case it's not modified).
func ToAutoscalerError(defaultType AutoscalerErrorType, err error) AutoscalerError {
	if e, ok := err.(AutoscalerError); ok {
		return e
	}
	return NewAutoscalerError(defaultType, err.Error())
}

// Error implements golang error interface
func (e autoscalerErrorImpl) Error() string {
	return e.msg
}

// Type returns the typ of AutoscalerError
func (e autoscalerErrorImpl) Type() AutoscalerErrorType {
	return e.errorType
}

// AddPrefix adds a prefix to error message.
// Returns the error it's called for convenient inline use.
// Example:
// if err := DoSomething(myObject); err != nil {
//	return err.AddPrefix("can't do something with %v: ", myObject)
// }
func (e autoscalerErrorImpl) AddPrefix(msg string, args ...interface{}) AutoscalerError {
	e.msg = fmt.Sprintf(msg, args...) + e.msg
	return e
}
