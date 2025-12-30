package grpc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLoggingInterceptor_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	interceptor := LoggingInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	resp, err := interceptor(context.Background(), "request", info, handler)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if resp != "response" {
		t.Errorf("expected response 'response', got %v", resp)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "host service call") {
		t.Errorf("expected log to contain 'host service call', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/test.Service/Method") {
		t.Errorf("expected log to contain method name, got: %s", logOutput)
	}
}

func TestLoggingInterceptor_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	interceptor := LoggingInterceptor(logger)

	testErr := status.Error(codes.NotFound, "not found")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, testErr
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(context.Background(), "request", info, handler)

	if err == nil {
		t.Error("expected error, got nil")
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "host service call failed") {
		t.Errorf("expected log to contain 'host service call failed', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "NotFound") {
		t.Errorf("expected log to contain gRPC code 'NotFound', got: %s", logOutput)
	}
}

func TestLoggingInterceptor_NonGRPCError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	interceptor := LoggingInterceptor(logger)

	testErr := errors.New("regular error")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, testErr
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	_, err := interceptor(context.Background(), "request", info, handler)

	if err == nil {
		t.Error("expected error, got nil")
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "host service call failed") {
		t.Errorf("expected log to contain 'host service call failed', got: %s", logOutput)
	}
}

// mockServerStream implements grpc.ServerStream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestLoggingStreamInterceptor_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	interceptor := LoggingStreamInterceptor(logger)

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return nil
	}

	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
	stream := &mockServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, info, handler)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "host service stream started") {
		t.Errorf("expected log to contain 'host service stream started', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "host service stream completed") {
		t.Errorf("expected log to contain 'host service stream completed', got: %s", logOutput)
	}
}

func TestLoggingStreamInterceptor_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	interceptor := LoggingStreamInterceptor(logger)

	testErr := status.Error(codes.Internal, "internal error")
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return testErr
	}

	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
	stream := &mockServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, info, handler)

	if err == nil {
		t.Error("expected error, got nil")
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "host service stream failed") {
		t.Errorf("expected log to contain 'host service stream failed', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Internal") {
		t.Errorf("expected log to contain gRPC code 'Internal', got: %s", logOutput)
	}
}
