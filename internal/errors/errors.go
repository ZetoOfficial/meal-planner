// Package apperr — доменные/приложенческие ошибки и их маппинг в HTTP (ARCHITECTURE §9).
package apperr

import "errors"

// Базовые классы ошибок для graceful fail (SPEC §6). TODO: расширить.
var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
	ErrConflict   = errors.New("conflict")
	ErrUnavailable = errors.New("temporarily unavailable")
)
