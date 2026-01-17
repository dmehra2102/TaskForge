package domain

import (
	"time"

	"github.com/google/uuid"
)

type TodoStatus int

const (
	StatusPending TodoStatus = iota + 1
	StatusInProgress
	StatusCompleted
	StatusArchived
)

type TodoPriority int

const (
	PriorityLow TodoPriority = iota + 1
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

type Todo struct {
	ID          string
	Title       string
	Description string
	Status      TodoStatus
	Priority    TodoPriority
	DueDate     *time.Time
	Tags        []string
	OwnerID     string
	AssignedTo  *string
	TenantID    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int64
}

// NewTodo creates a new todo with validation
func NewTodo(title, description, ownerID, tenantID string, priority TodoPriority) (*Todo, error) {
	if err := validateTitle(title); err != nil {
		return nil, err
	}

	if ownerID == "" {
		return nil, ErrInvalidOwnerId
	}

	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	now := time.Now().UTC()

	return &Todo{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		Priority:    priority,
		OwnerID:     ownerID,
		TenantID:    tenantID,
		Tags:        make([]string, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}, nil
}

// UpdateTitle updates the title with validation
func (t *Todo) UpdateTitle(title string) error {
	if err := validateTitle(title); err != nil {
		return err
	}
	t.Title = title
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// UpdateDescription updates the description
func (t *Todo) UpdateDescription(description string) error {
	if len(description) > 2000 {
		return ErrDescriptionTooLong
	}
	t.Description = description
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// UpdateStatus transitions the todo to a new status
func (t *Todo) UpdateStatus(newStatus TodoStatus) error {
	if !isValidStatusTransition(t.Status, newStatus) {
		return ErrInvalidStatusTransition
	}
	t.Status = newStatus
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// UpdatePriority changes the priority level
func (t *Todo) UpdatePriority(priority TodoPriority) error {
	if !isValidPriority(priority) {
		return ErrInvalidPriority
	}
	t.Priority = priority
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// SetDueDate sets or updates the due date
func (t *Todo) SetDueDate(dueDate *time.Time) error {
	if dueDate != nil && dueDate.Before(time.Now().UTC()) {
		return ErrDueDateInPast
	}
	t.DueDate = dueDate
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// AssignTo assigns the todo to a user
func (t *Todo) AssignTo(userID *string) error {
	t.AssignedTo = userID
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// AddTags adds tags to the todo
func (t *Todo) AddTags(tags []string) error {
	if len(t.Tags)+len(tags) > 20 {
		return ErrTooManyTags
	}
	t.Tags = append(t.Tags, tags...)
	t.UpdatedAt = time.Now().UTC()
	t.Version++
	return nil
}

// isValidStatusTransition checks if a status transition is allowed
func isValidStatusTransition(from, to TodoStatus) bool {
	validTransitions := map[TodoStatus][]TodoStatus{
		StatusPending: {
			StatusInProgress,
			StatusArchived,
		},
		StatusInProgress: {
			StatusArchived,
			StatusCompleted,
			StatusPending,
		},
		StatusCompleted: {
			StatusArchived,
			StatusPending,
		},
		StatusArchived: {
			StatusPending,
		},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}

	return false
}

func validateTitle(title string) error {
	if title == "" {
		return ErrEmptyTitle
	}
	if len(title) > 200 {
		return ErrTitleTooLong
	}
	return nil
}

func isValidPriority(p TodoPriority) bool {
	return p >= PriorityLow && p <= PriorityCritical
}

type ListFilter struct {
	TenantID      string
	OwnerID       *string
	AssignedTo    *string
	Statuses      []TodoStatus
	Priorities    []TodoPriority
	Tags          []string
	DueDateFrom   *time.Time
	DueDateTo     *time.Time
	SearchQuery   *string
	Page          int
	PageSize      int
	SortBy        string
	SortAscending bool
}

// Validates the filter
func (f *ListFilter) Validate() error {
	if f.TenantID == "" {
		return ErrInvalidTenantID
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return nil
}
