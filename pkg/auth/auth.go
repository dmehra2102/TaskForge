package auth

import (
	"context"
	"errors"
	"slices"

	"github.com/dmehra2102/TaskForge/internal/domain"
)

type contextKey string

const userContextKey contextKey = "user_context"

type UserContext struct {
	UserID   string
	TenantID string
	Roles    []string
}

// ContextWithUserContext adds user context to the context
func ContextWithUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, userCtx)
}

// UserContextFromContext extracts user context from the context
func UserContextFromContext(ctx context.Context) (*UserContext, error) {
	userCtx, ok := ctx.Value(userContextKey).(*UserContext)
	if !ok {
		return nil, errors.New("user context not found")
	}
	return userCtx, nil
}

type Authorizer struct{}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

func (a *Authorizer) CanCreate(userCtx *UserContext) bool {
	return hasRole(userCtx, "user") || hasRole(userCtx, "admin")
}

func (a *Authorizer) CanRead(userCtx *UserContext, todo *domain.Todo) bool {
	if hasRole(userCtx, "admin") && userCtx.TenantID == todo.TenantID {
		return true
	}

	// Users can read their own todos or todos assigned to them
	if userCtx.TenantID == todo.TenantID {
		if todo.OwnerID == userCtx.UserID {
			return true
		}
		if todo.AssignedTo != nil && *todo.AssignedTo == userCtx.UserID {
			return true
		}
	}

	return false
}

func (a *Authorizer) CanUpdate(userCtx *UserContext, todo *domain.Todo) bool {
	if hasRole(userCtx, "admin") && userCtx.TenantID == todo.TenantID {
		return true
	}

	if userCtx.TenantID == todo.TenantID {
		if todo.OwnerID == userCtx.UserID {
			return true
		}
		if todo.AssignedTo != nil && *todo.AssignedTo == userCtx.UserID {
			return true
		}
	}

	return false
}

func (a *Authorizer) CanDelete(userCtx *UserContext, todo *domain.Todo) bool {
	if hasRole(userCtx, "admin") && userCtx.TenantID == todo.TenantID {
		return true
	}

	// Only owners can delete their todos
	return userCtx.TenantID == todo.TenantID && todo.OwnerID == userCtx.UserID
}

func (a *Authorizer) CanReadAll(userCtx *UserContext) bool {
	return hasRole(userCtx, "admin")
}

func hasRole(userCtx *UserContext, role string) bool {
	return slices.Contains(userCtx.Roles, role)
}
