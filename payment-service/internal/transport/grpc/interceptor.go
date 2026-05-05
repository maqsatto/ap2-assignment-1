package grpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor implements gRPC unary server interceptor for logging
func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	// Log incoming request
	log.Printf("gRPC Request: method=%s", info.FullMethod)

	// Call the handler
	resp, err := handler(ctx, req)

	// Log response duration and status
	duration := time.Since(start)
	st := status.Convert(err)

	log.Printf("gRPC Response: method=%s status=%s duration=%v",
		info.FullMethod, st.Code(), duration)

	return resp, err
}
