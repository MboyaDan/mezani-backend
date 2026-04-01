package common

import "errors"

var (
	// =========================
	// Auth
	// =========================
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")

	// =========================
	// Resource
	// =========================
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")

	// =========================
	// Validation
	// =========================
	ErrInvalidInput    = errors.New("invalid input")
	ErrRequiredField   = errors.New("required field missing")
	ErrInvalidUUID     = errors.New("invalid uuid")
	ErrInvalidSession  = errors.New("invalid session")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionInactive = errors.New("session not active")
	ErrInvalidCustomer = errors.New("invalid customer session")

	// =========================
	// System
	// =========================
	ErrInternalServer = errors.New("internal server error")
	ErrDatabaseError  = errors.New("database error")
)
