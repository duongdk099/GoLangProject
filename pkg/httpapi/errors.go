// Package httpapi provides the shared, project-agnostic HTTP plumbing used by
// every BarterSwap feature package: sentinel errors, JSON helpers,
// middleware, authentication context, and routing. It has no dependency on
// any BarterSwap domain type, which is what makes it reusable across
// features without import cycles.
package httpapi

import "errors"

var (
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)
