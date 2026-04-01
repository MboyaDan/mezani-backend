package common

import "errors"

// Common errors that can be used across the entire application
var (
	// Authentication & Authorization
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")

	// Resource errors
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")

	// Validation errors
	ErrInvalidInput  = errors.New("invalid input")
	ErrRequiredField = errors.New("required field missing")

	// System errors
	ErrInternalServer = errors.New("internal server error")
	ErrDatabaseError  = errors.New("database error")
)
