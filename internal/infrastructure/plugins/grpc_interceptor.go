package plugins

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor returns a gRPC unary server interceptor that logs
// all requests and errors for host services called by plugins.
func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		duration := time.Since(start)

		// Log errors at error level, successful calls at debug level
		if err != nil {
			st, _ := status.FromError(err)
			logger.Error("host service call failed",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
				"error", err.Error(),
				"grpc_code", st.Code().String(),
			)
		} else {
			logger.Debug("host service call",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
			)
		}

		return resp, err
	}
}

// LoggingStreamInterceptor returns a gRPC stream server interceptor that logs
// stream lifecycle events and errors for host services called by plugins.
func LoggingStreamInterceptor(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		logger.Debug("host service stream started",
			"method", info.FullMethod,
		)

		err := handler(srv, ss)

		duration := time.Since(start)

		if err != nil {
			st, _ := status.FromError(err)
			logger.Error("host service stream failed",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
				"error", err.Error(),
				"grpc_code", st.Code().String(),
			)
		} else {
			logger.Debug("host service stream completed",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
			)
		}

		return err
	}
}
