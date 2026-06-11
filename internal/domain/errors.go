package domain

import "errors"

var (
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrValidation         = errors.New("validation failed")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
