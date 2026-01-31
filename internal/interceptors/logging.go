package interceptors

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	requestIDKey = "x-request-id"
)

func LoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := time.Now()

		requestID := getOrGenerateRequestID(ctx)

		// Add request ID to context
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDKey, requestID)

		// Log request
		logger.Info("gRPC request started",
			zap.String("method", info.FullMethod),
			zap.String("request_id", requestID),
		)

		resp, err = handler(ctx, req)

		duration := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.Duration("duration", duration),
				zap.String("cpde", st.Code().String()),
				zap.Error(err),
			)
		} else {
			logger.Info("gRPC request completed",
				zap.String("method", info.FullMethod),
				zap.String("request_id", requestID),
				zap.Duration("duration", duration),
			)
		}

		return resp, err
	}
}

func getOrGenerateRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.New().String()
	}

	requestIDs := md.Get(requestIDKey)
	if len(requestIDs) > 0 {
		return requestIDs[0]
	}

	return uuid.New().String()
}
