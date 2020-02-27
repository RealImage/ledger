package errors

import (
	"errors"
	"fmt"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrInternal   = errors.New("internal error")
	ErrConflict   = errors.New("conflict")
	ErrNotFound   = errors.New("not found")
)

// TODO: Do we really need this ApplicationError interface ?
// Can we just create named variables of type `error` like ErrorJSONInvalid, ErrorAccountConflict, etc.,

// ApplicationError is an interface to all application errors
// It provides functions to get the error code and error message
type ApplicationError interface {
	//TODO Should we embed log.Error in here and add a new function New(err errors, code, message string) *ApplicationError
	Error() string
	String() string
	ErrorCode() string
	ErrorMessage() string
}

// BaseApplicationError implements the `ApplicationError`
type BaseApplicationError struct {
	Message string
	Code    string
}

// ErrorCode returns the unique code of the error
func (e *BaseApplicationError) ErrorCode() string {
	return e.Code
}

// ErrorMessage returns the readable message of the error
func (e *BaseApplicationError) ErrorMessage() string {
	return e.Message
}

// Error returns string representation of error
func (e *BaseApplicationError) Error() string {
	return fmt.Sprintf("%v (%v)", e.Message, e.Code)
}

// String implementation supports logging
func (e *BaseApplicationError) String() string {
	return fmt.Sprintf("%v (%v)", e.Message, e.Code)
}
