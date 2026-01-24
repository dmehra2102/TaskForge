package app

import (
	todov1 "github.com/dmehra2102/TaskForge/api/proto/v1"
	"github.com/dmehra2102/TaskForge/internal/domain"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type TodoServiceServer struct {
	todov1.UnimplementedTodoServiceServer
	repo   domain.Repository
	logger *zap.Logger
	tracer trace.Tracer
	// authz *auth.Authorizer
}
