package main

import "errors"

var (
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)
