package apperr

type Code string

const (
	CodeValidation   Code = "validation_failed"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeUnauthorized Code = "unauthorized"
	CodeUnavailable  Code = "unavailable"
	CodeInternal     Code = "internal_error"
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
