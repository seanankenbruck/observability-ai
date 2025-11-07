// Package observability provides structured logging, metrics, and health checks
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	Level         LogLevel               `json:"level"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	Component     string                 `json:"component,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	Duration      time.Duration          `json:"duration_ms,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging with correlation IDs
type Logger struct {
	output    io.Writer
	minLevel  LogLevel
	component string
}

// NewLogger creates a new structured logger
func NewLogger(component string) *Logger {
	return &Logger{
		output:    os.Stdout,
		minLevel:  LevelInfo,
		component: component,
	}
}

// WithOutput sets the output writer for the logger
func (l *Logger) WithOutput(w io.Writer) *Logger {
	l.output = w
	return l
}

// WithLevel sets the minimum log level
func (l *Logger) WithLevel(level LogLevel) *Logger {
	l.minLevel = level
	return l
}

// log writes a structured log entry
func (l *Logger) log(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
		Component: l.component,
		Fields:    fields,
	}

	// Extract correlation ID from context
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		entry.CorrelationID = correlationID
	}

	// Extract user ID from context
	if userID := GetUserID(ctx); userID != "" {
		entry.UserID = userID
	}

	// Marshal and write
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	fmt.Fprintln(l.output, string(data))
}

// shouldLog checks if the log level should be logged
func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
	}

	return levels[level] >= levels[l.minLevel]
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(ctx, LevelDebug, message, fields)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(ctx, LevelInfo, message, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, fields map[string]interface{}) {
	l.log(ctx, LevelWarn, message, fields)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.log(ctx, LevelError, message, fields)
}

// WithOperation logs the start and end of an operation
func (l *Logger) WithOperation(ctx context.Context, operation string, fn func(context.Context) error) error {
	start := time.Now()
	correlationID := GetCorrelationID(ctx)
	if correlationID == "" {
		correlationID = uuid.New().String()
		ctx = WithCorrelationID(ctx, correlationID)
	}

	l.Info(ctx, fmt.Sprintf("Starting operation: %s", operation), map[string]interface{}{
		"operation": operation,
	})

	err := fn(ctx)
	duration := time.Since(start)

	fields := map[string]interface{}{
		"operation":   operation,
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		fields["error"] = err.Error()
		l.Error(ctx, fmt.Sprintf("Operation failed: %s", operation), err, fields)
		return err
	}

	l.Info(ctx, fmt.Sprintf("Operation completed: %s", operation), fields)
	return nil
}

// Context keys for storing values in context
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	userIDKey        contextKey = "user_id"
)

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// GetCorrelationID retrieves the correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// GetUserID retrieves the user ID from the context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(userIDKey).(string); ok {
		return id
	}
	return ""
}
