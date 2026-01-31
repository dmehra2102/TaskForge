package interceptors

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "code"},
	)

	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Histogram of gRPC request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	grpcActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grpc_active_requests",
			Help: "Number of active gRPC requests",
		},
		[]string{"method"},
	)
)

func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := time.Now()

		grpcActiveRequests.WithLabelValues(info.FullMethod).Inc()
		defer grpcActiveRequests.WithLabelValues(info.FullMethod).Dec()

		resp, err = handler(ctx, req)

		duration := time.Since(start).Seconds()
		grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		code := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			code = st.Code().String()
		}
		grpcRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()

		return resp, err
	}
}
