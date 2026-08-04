// Package models provides core domain entities and business logic errors.
// It is completely isolated from infrastructure, transport, and external dependencies.
package models

import "errors"

// Core business logic errors. These errors are returned by repositories and services,
// and should be mapped to appropriate HTTP status codes in the transport layer.
var (
	ErrQuantityInvalid = errors.New("quantity must be greater than zero")
	ErrTokenNotFound   = errors.New("token not found")
	ErrTokenExpired    = errors.New("token is expired")
	ErrTokenUsed       = errors.New("token has already been used")
	ErrForbidden       = errors.New("access denied: token belongs to another user")
	ErrStockDepleted   = errors.New("product is sold out")
	ErrInvalidStatus   = errors.New("invalid status value")
)
