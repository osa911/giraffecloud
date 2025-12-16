package service

import "errors"

// Sentinel errors for service layer
var (
	ErrValidation = errors.New("validation error")
	ErrConflict   = errors.New("conflict error")
	ErrNotFound   = errors.New("not found")
)
