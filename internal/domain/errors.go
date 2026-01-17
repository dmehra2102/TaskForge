package domain

import "errors"

var (
	// Validation Errors
	ErrEmptyTitle         = errors.New("title cannot be empty")
	ErrTitleTooLong       = errors.New("title exceeds 200 characters")
	ErrDescriptionTooLong = errors.New("description exceeds 2000 characters")
	ErrInvalidOwnerId     = errors.New("owner ID is required")
	ErrInvalidTenantID    = errors.New("tenant ID is required")
	ErrInvalidPriority    = errors.New("invalid priority value")
	ErrDueDateInPast      = errors.New("due date cannot be in the past")
	ErrTooManyTags        = errors.New("maximum 20 tags allowed")

	// Business logic errors
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrTodoNotFound            = errors.New("todo not found")
	ErrVersionMismatch         = errors.New("version mismatch - concurrent update detected")

	// Authorization errors
	ErrUnauthorized = errors.New("unauthorized access")
	ErrForbidden    = errors.New("forbidden - insufficient permissions")
)
