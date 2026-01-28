package app

import (
	"context"
	"fmt"
	"math"

	todov1 "github.com/dmehra2102/TaskForge/api/proto/v1"
	"github.com/dmehra2102/TaskForge/internal/domain"
	"github.com/dmehra2102/TaskForge/pkg/auth"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TodoServiceServer struct {
	todov1.UnimplementedTodoServiceServer
	repo   domain.Repository
	logger *zap.Logger
	tracer trace.Tracer
	authz  *auth.Authorizer
}

func NewTodoServiceServer(repo domain.Repository, logger *zap.Logger, authz *auth.Authorizer) *TodoServiceServer {
	return &TodoServiceServer{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("todo-service"),
		authz:  authz,
	}
}

func (s *TodoServiceServer) CreateTodo(ctx context.Context, req *todov1.CreateTodoRequest) (*todov1.CreateTodoResponse, error) {
	ctx, span := s.tracer.Start(ctx, "CreateTodo")
	defer span.End()

	userCtx, err := auth.UserContextFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	span.SetAttributes(
		attribute.String("user.id", userCtx.UserID),
		attribute.String("tenant.id", userCtx.TenantID),
	)

	// Validate Request
	if err := validateCreateRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// check authorization
	if !s.authz.CanCreate(userCtx) {
		return nil, status.Error(codes.PermissionDenied, "Insufficient permissions")
	}

	// create domain entity
	todo, err := domain.NewTodo(
		req.Title,
		req.Description,
		userCtx.UserID,
		userCtx.TenantID,
		mapProtoPriority(req.Priority),
	)
	if err != nil {
		s.logger.Error("failed to create todo entity",
			zap.Error(err),
			zap.String("user_id", userCtx.UserID),
		)
		return nil, mapDomainError(err)
	}

	// Optional fields
	if req.DueDate != nil {
		dueDate := req.DueDate.AsTime()
		if err := todo.SetDueDate(&dueDate); err != nil {
			return nil, mapDomainError(err)
		}
	}

	if len(req.Tags) > 0 {
		if err := todo.AddTags(req.Tags); err != nil {
			return nil, mapDomainError(err)
		}
	}

	if req.AssignedTo != "" {
		if err := todo.AssignTo(&req.AssignedTo); err != nil {
			return nil, mapDomainError(err)
		}
	}

	if err := s.repo.Create(ctx, todo); err != nil {
		s.logger.Error("failed to persist todo",
			zap.Error(err),
			zap.String("todo_id", todo.ID),
		)
		return nil, status.Error(codes.Internal, "failed to create todo")
	}

	s.logger.Info("todo created",
		zap.String("todo_id", todo.ID),
		zap.String("user_id", userCtx.UserID),
		zap.String("tenant_id", userCtx.TenantID),
	)

	return &todov1.CreateTodoResponse{
		Todo: mapDomainToProto(todo),
	}, nil
}

func (s *TodoServiceServer) GetTodo(ctx context.Context, req *todov1.GetTodoRequest) (*todov1.GetTodoResponse, error) {
	ctx, span := s.tracer.Start(ctx, "GetTodo")
	defer span.End()

	userCtx, err := auth.UserContextFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	span.SetAttributes(
		attribute.String("todo.id", req.Id),
		attribute.String("tenant.id", userCtx.TenantID),
	)

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "todo ID is required")
	}

	todo, err := s.repo.GetByID(ctx, req.Id, userCtx.TenantID)
	if err != nil {
		if err == domain.ErrTodoNotFound {
			return nil, status.Error(codes.NotFound, "todo not found")
		}
		s.logger.Error("failed to get todo",
			zap.Error(err),
			zap.String("todo_id", req.Id),
		)
		return nil, status.Error(codes.Internal, "failed to retrieve todo")
	}

	// Check authorization
	if !s.authz.CanRead(userCtx, todo) {
		return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
	}

	return &todov1.GetTodoResponse{
		Todo: mapDomainToProto(todo),
	}, nil
}

func (s *TodoServiceServer) UpdateTodo(ctx context.Context, req *todov1.UpdateTodoRequest) (*todov1.UpdateTodoResponse, error) {
	ctx, span := s.tracer.Start(ctx, "UpdateTodo")
	defer span.End()

	userCtx, err := auth.UserContextFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	existing, err := s.repo.GetByID(ctx, req.Id, userCtx.TenantID)
	if err != nil {
		if err == domain.ErrTodoNotFound {
			return nil, status.Error(codes.NotFound, "todo not found")
		}
		return nil, status.Error(codes.Internal, "failed to retrieve todo")
	}

	if !s.authz.CanUpdate(userCtx, existing) {
		return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
	}

	if err := applyFieldMaskUpdates(existing, req.Todo, req.UpdateMask); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		if err == domain.ErrVersionMismatch {
			return nil, status.Error(codes.Aborted, "concurrent update detected, please retry")
		}
		s.logger.Error("failed to update todo",
			zap.Error(err),
			zap.String("todo_id", req.Id),
		)
		return nil, status.Error(codes.Internal, "failed to update todo")
	}

	s.logger.Info("todo updated",
		zap.String("todo_id", req.Id),
		zap.String("user_id", userCtx.UserID),
	)

	return &todov1.UpdateTodoResponse{
		Todo: mapDomainToProto(existing),
	}, nil
}

func (s *TodoServiceServer) DeleteTodo(ctx context.Context, req *todov1.DeleteTodoRequest) (*todov1.DeleteTodoResponse, error) {
	ctx, span := s.tracer.Start(ctx, "DeleteTodo")
	defer span.End()

	userCtx, err := auth.UserContextFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	todo, err := s.repo.GetByID(ctx, req.Id, userCtx.TenantID)
	if err != nil {
		if err == domain.ErrTodoNotFound {
			return nil, status.Error(codes.NotFound, "todo not found")
		}
		return nil, status.Error(codes.Internal, "failed to retrieve todo")
	}

	if !s.authz.CanDelete(userCtx, todo) {
		return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
	}

	if err := s.repo.Delete(ctx, req.Id, userCtx.TenantID); err != nil {
		s.logger.Error("failed to delete todo",
			zap.Error(err),
			zap.String("todo_id", req.Id),
		)
		return nil, status.Error(codes.Internal, "failed to delete todo")
	}

	s.logger.Info("todo deleted",
		zap.String("todo_id", req.Id),
		zap.String("user_id", userCtx.UserID),
	)

	return &todov1.DeleteTodoResponse{
		Success: true,
	}, nil
}

func (s *TodoServiceServer) ListTodos(ctx context.Context, req *todov1.ListTodosRequest) (*todov1.ListTodosResponse, error) {
	ctx, span := s.tracer.Start(ctx, "ListTodos")
	defer span.End()

	userCtx, err := auth.UserContextFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	filter := &domain.ListFilter{
		TenantID:      userCtx.TenantID,
		Page:          int(req.Page),
		PageSize:      int(req.PageSize),
		SortBy:        req.SortBy,
		SortAscending: req.SortOrder == todov1.SortOrder_SORT_ORDER_ASC,
	}

	if !s.authz.CanReadAll(userCtx) {
		filter.OwnerID = &userCtx.UserID
	}

	if len(req.StatusFilter) > 0 {
		filter.Statuses = make([]domain.TodoStatus, len(req.StatusFilter))
		for i, s := range req.StatusFilter {
			filter.Statuses[i] = mapProtoStatus(s)
		}
	}

	if len(req.PriorityFilter) > 0 {
		filter.Priorities = make([]domain.TodoPriority, len(req.PriorityFilter))
		for i, p := range req.PriorityFilter {
			filter.Priorities[i] = mapProtoPriority(p)
		}
	}

	if len(req.TagsFilter) > 0 {
		filter.Tags = req.TagsFilter
	}

	if req.AssignedToFilter != "" {
		filter.AssignedTo = &req.AssignedToFilter
	}

	if req.DueDateFrom != nil {
		from := req.DueDateFrom.AsTime()
		filter.DueDateFrom = &from
	}

	if req.DueDateTo != nil {
		to := req.DueDateTo.AsTime()
		filter.DueDateTo = &to
	}

	if req.SearchQuery != "" {
		filter.SearchQuery = &req.SearchQuery
	}

	if err := filter.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	todos, totalCount, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list todos",
			zap.Error(err),
			zap.String("tenant_id", userCtx.TenantID),
		)
		return nil, status.Error(codes.Internal, "failed to list todos")
	}

	// Build response
	protoTodos := make([]*todov1.Todo, len(todos))
	for i, todo := range todos {
		protoTodos[i] = mapDomainToProto(todo)
	}

	totalPages := int32(math.Ceil(float64(totalCount) / float64(filter.PageSize)))

	pageInfo := &todov1.PageInfo{
		Page:       int32(filter.Page),
		PageSize:   int32(filter.PageSize),
		TotalItems: totalCount,
		TotalPages: totalPages,
		HasNext:    filter.Page < int(totalPages),
		HasPrev:    filter.Page > 1,
	}

	return &todov1.ListTodosResponse{
		Todos:    protoTodos,
		PageInfo: pageInfo,
	}, nil
}

func validateCreateRequest(req *todov1.CreateTodoRequest) error {
	if req.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(req.Title) > 200 {
		return fmt.Errorf("title exceeds maximum length")
	}
	return nil
}

func mapDomainToProto(todo *domain.Todo) *todov1.Todo {
	proto := &todov1.Todo{
		Id:          todo.ID,
		Title:       todo.Title,
		Description: todo.Description,
		Status:      mapDomainStatus(todo.Status),
		Priority:    mapDomainPriority(todo.Priority),
		Tags:        todo.Tags,
		OwnerId:     todo.OwnerID,
		TenantId:    todo.TenantID,
		CreatedAt:   timestamppb.New(todo.CreatedAt),
		UpdatedAt:   timestamppb.New(todo.UpdatedAt),
		Version:     todo.Version,
	}

	if todo.DueDate != nil {
		proto.DueDate = timestamppb.New(*todo.DueDate)
	}

	if todo.AssignedTo != nil {
		proto.AssignedTo = *todo.AssignedTo
	}

	return proto
}

func mapDomainStatus(s domain.TodoStatus) todov1.TodoStatus {
	switch s {
	case domain.StatusPending:
		return todov1.TodoStatus_TODO_STATUS_PENDING
	case domain.StatusInProgress:
		return todov1.TodoStatus_TODO_STATUS_IN_PROGRESS
	case domain.StatusCompleted:
		return todov1.TodoStatus_TODO_STATUS_COMPLETED
	case domain.StatusArchived:
		return todov1.TodoStatus_TODO_STATUS_ARCHIVED
	default:
		return todov1.TodoStatus_TODO_STATUS_UNSPECIFIED
	}
}

func mapProtoStatus(s todov1.TodoStatus) domain.TodoStatus {
	switch s {
	case todov1.TodoStatus_TODO_STATUS_PENDING:
		return domain.StatusPending
	case todov1.TodoStatus_TODO_STATUS_IN_PROGRESS:
		return domain.StatusInProgress
	case todov1.TodoStatus_TODO_STATUS_COMPLETED:
		return domain.StatusCompleted
	case todov1.TodoStatus_TODO_STATUS_ARCHIVED:
		return domain.StatusArchived
	default:
		return domain.StatusPending
	}
}

func mapDomainPriority(p domain.TodoPriority) todov1.TodoPriority {
	switch p {
	case domain.PriorityLow:
		return todov1.TodoPriority_TODO_PRIORITY_LOW
	case domain.PriorityMedium:
		return todov1.TodoPriority_TODO_PRIORITY_MEDIUM
	case domain.PriorityHigh:
		return todov1.TodoPriority_TODO_PRIORITY_HIGH
	case domain.PriorityCritical:
		return todov1.TodoPriority_TODO_PRIORITY_CRITICAL
	default:
		return todov1.TodoPriority_TODO_PRIORITY_UNSPECIFIED
	}
}

func mapProtoPriority(p todov1.TodoPriority) domain.TodoPriority {
	switch p {
	case todov1.TodoPriority_TODO_PRIORITY_LOW:
		return domain.PriorityLow
	case todov1.TodoPriority_TODO_PRIORITY_HIGH:
		return domain.PriorityHigh
	case todov1.TodoPriority_TODO_PRIORITY_CRITICAL:
		return domain.PriorityCritical
	default:
		return domain.PriorityMedium
	}
}

func mapDomainError(err error) error {
	switch err {
	case domain.ErrEmptyTitle, domain.ErrTitleTooLong, domain.ErrDescriptionTooLong,
		domain.ErrInvalidPriority, domain.ErrDueDateInPast, domain.ErrTooManyTags:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrInvalidStatusTransition:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrTodoNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrVersionMismatch:
		return status.Error(codes.Aborted, err.Error())
	case domain.ErrUnauthorized:
		return status.Error(codes.Unauthenticated, err.Error())
	case domain.ErrForbidden:
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func applyFieldMaskUpdates(existing *domain.Todo, updates *todov1.Todo, mask *fieldmaskpb.FieldMask) error {
	if mask == nil || len(mask.Paths) == 0 {
		// Update all fields if no mask
		return updateAllFields(existing, updates)
	}

	for _, path := range mask.Paths {
		switch path {
		case "title":
			if err := existing.UpdateTitle(updates.Title); err != nil {
				return err
			}
		case "description":
			if err := existing.UpdateDescription(updates.Description); err != nil {
				return err
			}
		case "priority":
			if err := existing.UpdatePriority(mapProtoPriority(updates.Priority)); err != nil {
				return err
			}
		case "status":
			if err := existing.UpdateStatus(mapProtoStatus(updates.Status)); err != nil {
				return err
			}
		case "due_date":
			if updates.DueDate != nil {
				dueDate := updates.DueDate.AsTime()
				if err := existing.SetDueDate(&dueDate); err != nil {
					return err
				}
			}
		case "assigned_to":
			if updates.AssignedTo != "" {
				if err := existing.AssignTo(&updates.AssignedTo); err != nil {
					return err
				}
			}
		case "tags":
			if len(updates.Tags) > 0 {
				if err := existing.AddTags(updates.Tags); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func updateAllFields(existing *domain.Todo, updates *todov1.Todo) error {
	if err := existing.UpdateTitle(updates.Title); err != nil {
		return err
	}
	if err := existing.UpdateDescription(updates.Description); err != nil {
		return err
	}
	if err := existing.UpdatePriority(mapProtoPriority(updates.Priority)); err != nil {
		return err
	}
	return nil
}
