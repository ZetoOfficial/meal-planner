package apperr

import (
	"errors"
)

type Code string

const (
	CodeValidation     Code = "validation_failed"
	CodeNotFound       Code = "not_found"
	CodeConflict       Code = "conflict"
	CodeUnauthorized   Code = "unauthorized"
	CodeUnavailable    Code = "unavailable"
	CodeImpossiblePlan Code = "impossible_plan"
	CodeInternal       Code = "internal_error"
)

var (
	ErrValidation     = errors.New(string(CodeValidation))
	ErrNotFound       = errors.New(string(CodeNotFound))
	ErrConflict       = errors.New(string(CodeConflict))
	ErrUnauthorized   = errors.New(string(CodeUnauthorized))
	ErrUnavailable    = errors.New(string(CodeUnavailable))
	ErrImpossiblePlan = errors.New(string(CodeImpossiblePlan))
	ErrInternal       = errors.New(string(CodeInternal))
)

type Error struct {
	Code    Code
	Message string
	Details []string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == sentinel(e.Code)
}

func Validation(message string, details ...string) error {
	return New(CodeValidation, message, details...)
}

func NotFound(resource string) error {
	return New(CodeNotFound, resource+" not found")
}

func Conflict(message string, details ...string) error {
	return New(CodeConflict, message, details...)
}

func Unauthorized(message string) error {
	return New(CodeUnauthorized, message)
}

func Unavailable(message string, cause error) error {
	return &Error{Code: CodeUnavailable, Message: message, Cause: cause}
}

func New(code Code, message string, details ...string) error {
	return &Error{Code: code, Message: message, Details: details}
}

type UnmetConstraint struct {
	RoleTag string
	Reason  string
}

type ImpossiblePlanError struct {
	Message          string
	UnmetConstraints []UnmetConstraint
	Cause            error
}

func ImpossiblePlan(message string, constraints ...UnmetConstraint) error {
	return &ImpossiblePlanError{
		Message:          message,
		UnmetConstraints: constraints,
	}
}

func (e *ImpossiblePlanError) Error() string {
	if e == nil || e.Message == "" {
		return string(CodeImpossiblePlan)
	}
	return e.Message
}

func (e *ImpossiblePlanError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *ImpossiblePlanError) Is(target error) bool {
	return target == ErrImpossiblePlan
}

func sentinel(code Code) error {
	switch code {
	case CodeValidation:
		return ErrValidation
	case CodeNotFound:
		return ErrNotFound
	case CodeConflict:
		return ErrConflict
	case CodeUnauthorized:
		return ErrUnauthorized
	case CodeUnavailable:
		return ErrUnavailable
	case CodeImpossiblePlan:
		return ErrImpossiblePlan
	case CodeInternal:
		return ErrInternal
	default:
		return nil
	}
}
