package servicebus

import (
	"fmt"
	"github.com/Azure/azure-amqp-common-go/rpc"
	"reflect"
)

type (
	// ErrMissingField indicates that an expected property was missing from an AMQP message. This should only be
	// encountered when there is an error with this library, or the server has altered its behavior unexpectedly.
	ErrMissingField string

	// ErrIncorrectType indicates that type assertion failed. This should only be encountered when there is an error
	// with this library, or the server has altered its behavior unexpectedly.
	ErrIncorrectType struct {
		Key          string
		ExpectedType reflect.Type
		ActualValue  interface{}
	}

	// ErrAMQP indicates that the server communicated an AMQP error with a particular
	ErrAMQP rpc.Response

	// ErrNoMessages is returned when an operation returned no messages. It is not indicative that there will not be
	// more messages in the future.
	ErrNoMessages struct{}
)

func (e ErrMissingField) Error() string {
	return fmt.Sprintf("missing value %q", string(e))
}

// NewErrIncorrectType lets you skip using the `reflect` package. Just provide a variable of the desired type as
// 'expected'.
func newErrIncorrectType(key string, expected, actual interface{}) ErrIncorrectType {
	return ErrIncorrectType{
		Key:          key,
		ExpectedType: reflect.TypeOf(expected),
		ActualValue:  actual,
	}
}

func (e ErrIncorrectType) Error() string {
	return fmt.Sprintf(
		"value at %q was expected to be of type %q but was actually of type %q",
		e.Key,
		e.ExpectedType,
		reflect.TypeOf(e.ActualValue))
}

func (e ErrAMQP) Error() string {
	return fmt.Sprintf("server says (%d) %s", e.Code, e.Description)
}

func (e ErrNoMessages) Error() string {
	return "no messages available"
}
