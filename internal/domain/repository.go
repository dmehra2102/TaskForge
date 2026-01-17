package domain

import "context"

// Repository defines the contract for todo persistence
type Repository interface {
	// Create persists a new todo
	Create(ctx context.Context, todo *Todo) error

	// GetByID retrieves a todo by ID
	GetByID(ctx context.Context, id, tenantID string) (*Todo, error)

	// Update updates an existing todo with optimistic locking
	Update(ctx context.Context, todo *Todo) error

	// Delete soft-deletes a todo
	Delete(ctx context.Context, id, tenantID string) error

	// List retrieves todos with filtering and pagination
	List(ctx context.Context, filter *ListFilter) ([]*Todo, int64, error)

	// UpdateStatus updates only the status field
	UpdateStatus(ctx context.Context, id, tenantID string, status TodoStatus, version int64) (*Todo, error)

	// BatchCreate creates multiple todos in a transaction
	BatchCreate(ctx context.Context, todos []*Todo) error
}

// PageResult contains paginated results
type PageResult struct {
	Items      []*Todo
	TotalItems int64
	Page       int
	PageSize   int
	TotalPages int
}
