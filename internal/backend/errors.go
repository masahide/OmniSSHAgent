package backend

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorUnavailable ErrorKind = "unavailable"
	ErrorTimeout     ErrorKind = "timeout"
	ErrorProtocol    ErrorKind = "protocol"
)

type Error struct {
	Kind      ErrorKind
	Operation string
	Err       error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s backend %s: %v", e.Kind, e.Operation, e.Err)
}
func (e *Error) Unwrap() error { return e.Err }

func IsKind(err error, kind ErrorKind) bool {
	var backendError *Error
	return errors.As(err, &backendError) && backendError.Kind == kind
}
