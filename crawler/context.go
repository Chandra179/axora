package crawler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type ContextKey string

const (
	ContextIDKey ContextKey = "context_id"
	IPKey        ContextKey = "ip"
)

// GetContextLogger creates a logger with context information
func GetContextLogger(ctx context.Context, baseLogger *zap.Logger) *zap.Logger {
	logger := baseLogger

	if contextID := ctx.Value(ContextIDKey); contextID != nil {
		logger = logger.With(zap.String("context_id", contextID.(string)))
	}

	if ip := ctx.Value(IPKey); ip != nil {
		logger = logger.With(zap.String("ip", ip.(string)))
	}

	return logger
}

// WithContextID adds a context ID to the context
func WithContextID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextIDKey, id)
}

// WithIP adds an IP address to the context
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, IPKey, ip)
}

// GenerateContextID generates a unique context ID with a prefix
func GenerateContextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// GetContextID retrieves the context ID from context
func GetContextID(ctx context.Context) string {
	if contextID := ctx.Value(ContextIDKey); contextID != nil {
		return contextID.(string)
	}
	return ""
}

// GetContextIP retrieves the IP address from context
func GetContextIP(ctx context.Context) string {
	if ip := ctx.Value(IPKey); ip != nil {
		return ip.(string)
	}
	return ""
}
