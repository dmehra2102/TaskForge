package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dmehra2102/TaskForge/internal/domain"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const queryTimeout = 5 * time.Second

type PostgresRepository struct {
	db     *sql.DB
	tracer trace.Tracer
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		tracer: otel.Tracer("postgres-repository"),
	}
}

func (r *PostgresRepository) Create(ctx context.Context, todo *domain.Todo) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.Create")
	defer span.End()

	span.SetAttributes(
		attribute.String("todo.id", todo.ID),
		attribute.String("tenant.id", todo.TenantID),
	)

	query := `
		INSERT INTO todos (
			id, title, description, status, priority, due_date, tags, owner_id, assigned_to,
			tenant_id, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.ExecContext(ctx, query,
		todo.ID,
		todo.Title,
		todo.Description,
		todo.Status,
		todo.Priority,
		todo.DueDate,
		pq.Array(todo.Tags),
		todo.OwnerID,
		todo.AssignedTo,
		todo.TenantID,
		todo.CreatedAt,
		todo.UpdatedAt,
		todo.Version,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create todo: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id, tenantID string) (*domain.Todo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.GetByID")
	defer span.End()

	span.SetAttributes(
		attribute.String("todo.id", id),
		attribute.String("tenant.id", tenantID),
	)

	query := `
		SELECT id, title, description, status, priority, due_date, tags, owner_id, assigned_to, tenant_id, created_at, updated_at, version 
		FROM todos
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`

	todo := &domain.Todo{}
	var tags pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id, tenantID).Scan(
		&todo.ID,
		&todo.Title,
		&todo.Description,
		&todo.Status,
		&todo.Priority,
		&todo.DueDate,
		&tags,
		&todo.OwnerID,
		&todo.AssignedTo,
		&todo.TenantID,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&todo.Version,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetAttributes(attribute.Bool("not_found", true))
			return nil, domain.ErrTodoNotFound
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	todo.Tags = tags
	return todo, nil
}

func (r *PostgresRepository) Update(ctx context.Context, todo *domain.Todo) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.Update")
	defer span.End()

	span.SetAttributes(
		attribute.String("todo.id", todo.ID),
		attribute.String("tenant.id", todo.TenantID),
		attribute.Int64("version", todo.Version),
	)

	// Optimistic locking: update only if version matches
	query := `
		UPDATE todos
		SET title = $1, description = $2, status = $3, priority = $4, due_date = $5, tags = $6, assigned_to = $7, updated_at = $8, version = version + 1 
		WHERE id = $9 AND tenant_id = $10 AND version = $11 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query,
		todo.Title,
		todo.Description,
		todo.Status,
		todo.Priority,
		todo.DueDate,
		pq.Array(todo.Tags),
		todo.AssignedTo,
		time.Now().UTC(),
		todo.ID,
		todo.TenantID,
		todo.Version,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update todo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetAttributes(attribute.Bool("version_mismatch", true))
		return domain.ErrVersionMismatch
	}

	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id, tenantID string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.Delete")
	defer span.End()

	// Soft delete
	query := `
		UPDATE todos
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND tenant_id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), id, tenantID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrTodoNotFound
	}

	return nil
}

func (r *PostgresRepository) List(ctx context.Context, filter *domain.ListFilter) ([]*domain.Todo, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.List")
	defer span.End()

	where, args := buildWhereClause(filter)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM todos WHERE %s", where)

	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to count todos: %w", err)
	}

	// Build ORDER BY clause
	orderBy := buildOrderByClause(filter)

	// Calculate offset
	offset := (filter.Page - 1) * filter.PageSize

	// Query with pagination
	query := fmt.Sprintf(`
		SELECT id, title, description, status, priority, due_date, tags, owner_id, assigned_to, tenant_id, created_at, updated_at, version
		FROM todos
		WHERE %s
		%s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, len(args)+1, len(args)+2)

	args = append(args, filter.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to list todos: %w", err)
	}
	defer rows.Close()

	todos := make([]*domain.Todo, 0)
	for rows.Next() {
		todo := &domain.Todo{}
		var tags pq.StringArray

		err := rows.Scan(
			&todo.ID,
			&todo.Title,
			&todo.Description,
			&todo.Status,
			&todo.Priority,
			&todo.DueDate,
			&tags,
			&todo.OwnerID,
			&todo.AssignedTo,
			&todo.TenantID,
			&todo.CreatedAt,
			&todo.UpdatedAt,
			&todo.Version,
		)
		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to scan todo: %w", err)
		}

		todo.Tags = tags
		todos = append(todos, todo)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("error iterating todos: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("total_count", totalCount),
		attribute.Int("returned_count", len(todos)),
	)

	return todos, totalCount, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id, tenantID string, status domain.TodoStatus, version int64) (*domain.Todo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.UpdateStatus")
	defer span.End()

	query := `
		UPDATE todos
		SET status = $1, updated_at = $2, version = version + 1
		WHERE id = $3 AND tenant_id = $4 AND version = $5 AND deleted_at IS NULL
		RETURNING id, title, description, status, priority, due_date, tags, owner_id, assigned_to, tenant_id, created_at, updated_at, version
	`

	todo := &domain.Todo{}
	var tags pq.StringArray

	err := r.db.QueryRowContext(ctx, query, status, time.Now().UTC(), id, tenantID, version).Scan(
		&todo.ID,
		&todo.Title,
		&todo.Description,
		&todo.Status,
		&todo.Priority,
		&todo.DueDate,
		&tags,
		&todo.OwnerID,
		&todo.AssignedTo,
		&todo.TenantID,
		&todo.CreatedAt,
		&todo.UpdatedAt,
		&todo.Version,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrVersionMismatch
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	todo.Tags = tags
	return todo, nil
}

func (r *PostgresRepository) BatchCreate(ctx context.Context, todos []*domain.Todo) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	ctx, span := r.tracer.Start(ctx, "repository.BatchCreate")
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO todos (
			id, title, description, status, priority,
			due_date, tags, owner_id, assigned_to, tenant_id,
			created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, todo := range todos {
		_, err := stmt.ExecContext(ctx,
			todo.ID,
			todo.Title,
			todo.Description,
			todo.Status,
			todo.Priority,
			todo.DueDate,
			pq.Array(todo.Tags),
			todo.OwnerID,
			todo.AssignedTo,
			todo.TenantID,
			todo.CreatedAt,
			todo.UpdatedAt,
			todo.Version,
		)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to insert todo %s: %w", todo.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetAttributes(attribute.Int("batch_size", len(todos)))
	return nil
}

func buildWhereClause(filter *domain.ListFilter) (string, []any) {
	conditions := []string{"tenant_id = $1", "delete_at IS NULL"}
	args := []any{filter.TenantID}
	argCount := 1

	if filter.OwnerID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argCount))
		args = append(args, *filter.OwnerID)
	}

	if filter.AssignedTo != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("assigned_to = $%d", argCount))
		args = append(args, *filter.AssignedTo)
	}

	if len(filter.Statuses) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argCount))
		statusInts := make([]int, len(filter.Statuses))
		for i, s := range filter.Statuses {
			statusInts[i] = int(s)
		}
		args = append(args, pq.Array(statusInts))
	}

	if len(filter.Priorities) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("priority = ANY($%d)", argCount))
		priorityInts := make([]int, len(filter.Priorities))
		for i, p := range filter.Priorities {
			priorityInts[i] = int(p)
		}
		args = append(args, pq.Array(priorityInts))
	}

	if len(filter.Tags) > 0 {
		argCount++
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argCount))
		args = append(args, pq.Array(filter.Tags))
	}

	if filter.DueDateFrom != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("due_date >= $%d", argCount))
		args = append(args, *filter.DueDateFrom)
	}

	if filter.DueDateTo != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("due_date <= $%d", argCount))
		args = append(args, *filter.DueDateTo)
	}

	if filter.SearchQuery != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+*filter.SearchQuery+"%")
	}

	return strings.Join(conditions, " AND "), args
}

func buildOrderByClause(filter *domain.ListFilter) string {
	validSortFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"due_date":   true,
		"priority":   true,
		"status":     true,
		"title":      true,
	}

	sortBy := "created_at"
	if validSortFields[filter.SortBy] {
		sortBy = filter.SortBy
	}

	order := "DESC"
	if filter.SortAscending {
		order = "ASC"
	}

	return fmt.Sprintf("ORDER BY %s %s", sortBy, order)
}
