// Package errors provides enhanced error types with helpful context and suggestions
package errors

import (
	"fmt"
	"strings"
)

// ErrorCode represents a unique error identifier
type ErrorCode string

const (
	// Query processing errors
	ErrCodeIntentClassification ErrorCode = "INTENT_CLASSIFICATION_FAILED"
	ErrCodeEmbeddingGeneration  ErrorCode = "EMBEDDING_GENERATION_FAILED"
	ErrCodePromptBuilding       ErrorCode = "PROMPT_BUILD_FAILED"
	ErrCodeQueryGeneration      ErrorCode = "QUERY_GENERATION_FAILED"
	ErrCodeSafetyValidation     ErrorCode = "SAFETY_VALIDATION_FAILED"

	// Safety check errors
	ErrCodeForbiddenMetric    ErrorCode = "FORBIDDEN_METRIC"
	ErrCodeExcessiveTimeRange ErrorCode = "EXCESSIVE_TIME_RANGE"
	ErrCodeHighCardinality    ErrorCode = "HIGH_CARDINALITY"
	ErrCodeExpensiveOperation ErrorCode = "EXPENSIVE_OPERATION"
	ErrCodeTooManyNested      ErrorCode = "TOO_MANY_NESTED_OPS"

	// Database errors
	ErrCodeDatabaseConnection ErrorCode = "DATABASE_CONNECTION_FAILED"
	ErrCodeDatabaseQuery      ErrorCode = "DATABASE_QUERY_FAILED"
	ErrCodeServiceNotFound    ErrorCode = "SERVICE_NOT_FOUND"

	// Authentication errors
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeTokenCreation      ErrorCode = "TOKEN_CREATION_FAILED"
	ErrCodeSessionCreation    ErrorCode = "SESSION_CREATION_FAILED"
	ErrCodeNotAuthenticated   ErrorCode = "NOT_AUTHENTICATED"
	ErrCodeInsufficientPerms  ErrorCode = "INSUFFICIENT_PERMISSIONS"

	// Input validation errors
	ErrCodeInvalidInput    ErrorCode = "INVALID_INPUT"
	ErrCodeMissingRequired ErrorCode = "MISSING_REQUIRED_FIELD"
	ErrCodeInvalidDuration ErrorCode = "INVALID_DURATION"

	// Cache errors
	ErrCodeCacheRead  ErrorCode = "CACHE_READ_FAILED"
	ErrCodeCacheWrite ErrorCode = "CACHE_WRITE_FAILED"
)

// EnhancedError represents an error with additional context and helpful information
type EnhancedError struct {
	Code          ErrorCode              `json:"code"`
	Message       string                 `json:"message"`
	Details       string                 `json:"details,omitempty"`
	Suggestion    string                 `json:"suggestion,omitempty"`
	Documentation string                 `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Cause         error                  `json:"-"`
}

// Error implements the error interface
func (e *EnhancedError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", e.Code, e.Message))
	if e.Details != "" {
		sb.WriteString(fmt.Sprintf(": %s", e.Details))
	}
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf(" (cause: %v)", e.Cause))
	}
	return sb.String()
}

// Unwrap returns the underlying error for error chain unwrapping
func (e *EnhancedError) Unwrap() error {
	return e.Cause
}

// UserMessage returns a user-friendly error message with suggestions
func (e *EnhancedError) UserMessage() string {
	var sb strings.Builder
	sb.WriteString(e.Message)

	if e.Details != "" {
		sb.WriteString(fmt.Sprintf("\n\nDetails: %s", e.Details))
	}

	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("\n\nSuggestion: %s", e.Suggestion))
	}

	if e.Documentation != "" {
		sb.WriteString(fmt.Sprintf("\n\nLearn more: %s", e.Documentation))
	}

	return sb.String()
}

// New creates a new EnhancedError
func New(code ErrorCode, message string) *EnhancedError {
	return &EnhancedError{
		Code:     code,
		Message:  message,
		Metadata: make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with enhanced context
func Wrap(err error, code ErrorCode, message string) *EnhancedError {
	return &EnhancedError{
		Code:     code,
		Message:  message,
		Cause:    err,
		Metadata: make(map[string]interface{}),
	}
}

// WithDetails adds detailed information about the error
func (e *EnhancedError) WithDetails(details string) *EnhancedError {
	e.Details = details
	return e
}

// WithSuggestion adds a suggestion on how to fix the error
func (e *EnhancedError) WithSuggestion(suggestion string) *EnhancedError {
	e.Suggestion = suggestion
	return e
}

// WithMetadata adds additional metadata to the error
func (e *EnhancedError) WithMetadata(key string, value interface{}) *EnhancedError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// Common error constructors with pre-configured messages

// NewIntentClassificationError creates an error for intent classification failures
func NewIntentClassificationError(err error, query string) *EnhancedError {
	return Wrap(err, ErrCodeIntentClassification, "Failed to classify query intent").
		WithDetails(fmt.Sprintf("Could not determine the intent of query: '%s'", query)).
		WithSuggestion("Try rephrasing your query to be more specific. For example: 'Show error rate for my-service' or 'What is the latency for api-gateway?'")
}

// NewEmbeddingGenerationError creates an error for embedding generation failures
func NewEmbeddingGenerationError(err error) *EnhancedError {
	return Wrap(err, ErrCodeEmbeddingGeneration, "Failed to generate query embedding").
		WithDetails("The AI service was unable to process your query for semantic search").
		WithSuggestion("This is typically a temporary issue. Please try your query again in a moment.").
		WithMetadata("retryable", true)
}

// NewQueryGenerationError creates an error for PromQL generation failures
func NewQueryGenerationError(err error) *EnhancedError {
	return Wrap(err, ErrCodeQueryGeneration, "Failed to generate PromQL query").
		WithDetails("The AI was unable to convert your natural language query to PromQL").
		WithSuggestion("Try simplifying your query or being more specific about the metrics you want to query.")
}

// NewForbiddenMetricError creates an error for forbidden metric access
func NewForbiddenMetricError(pattern string) *EnhancedError {
	return New(ErrCodeForbiddenMetric, "Query contains forbidden metric").
		WithDetails(fmt.Sprintf("The query attempts to access metrics matching the forbidden pattern: %s", pattern)).
		WithSuggestion("Metrics containing sensitive information (secrets, passwords, tokens, keys) cannot be queried directly. Please contact your administrator if you need access.")
}

// NewExcessiveTimeRangeError creates an error for excessive time ranges
func NewExcessiveTimeRangeError(timeRange string, maxAllowed string) *EnhancedError {
	return New(ErrCodeExcessiveTimeRange, "Query time range exceeds maximum allowed").
		WithDetails(fmt.Sprintf("The query requests data for %s, which exceeds the maximum allowed range of %s", timeRange, maxAllowed)).
		WithSuggestion(fmt.Sprintf("Please reduce the time range to %s or less. For historical analysis, consider using aggregated data or downsampled metrics.", maxAllowed))
}

// NewHighCardinalityError creates an error for high cardinality queries
func NewHighCardinalityError() *EnhancedError {
	return New(ErrCodeHighCardinality, "Query may produce high cardinality results").
		WithDetails("The query structure suggests it could return an excessive number of time series").
		WithSuggestion("Add more specific label filters or use aggregation functions like sum(), avg(), or max(). Avoid queries that group by no labels or use 'without ()'.")
}

// NewExpensiveOperationError creates an error for expensive operations
func NewExpensiveOperationError(operation string) *EnhancedError {
	return New(ErrCodeExpensiveOperation, "Query contains potentially expensive operation").
		WithDetails(fmt.Sprintf("The query uses the '%s' operation which can be resource-intensive", operation)).
		WithSuggestion("Consider rewriting your query to avoid expensive operations like 'group_left', 'group_right', or 'absent()'. Use simpler aggregations when possible.")
}

// NewServiceNotFoundError creates an error for service not found
func NewServiceNotFoundError(serviceName string) *EnhancedError {
	return New(ErrCodeServiceNotFound, "Service not found").
		WithDetails(fmt.Sprintf("No service found with name: %s", serviceName)).
		WithSuggestion("Check the service name for typos. Use the /api/v1/services endpoint to see all available services, or /api/v1/services/search?q=<name> to search for services.").
		WithMetadata("service_name", serviceName)
}

// NewInvalidCredentialsError creates an error for authentication failures
func NewInvalidCredentialsError() *EnhancedError {
	return New(ErrCodeInvalidCredentials, "Invalid username or password").
		WithDetails("Authentication failed with the provided credentials").
		WithSuggestion("Please check your username and password and try again. If you've forgotten your password, contact your administrator.")
}

// NewTokenCreationError creates an error for token creation failures
func NewTokenCreationError(err error) *EnhancedError {
	return Wrap(err, ErrCodeTokenCreation, "Failed to create authentication token").
		WithDetails("The system was unable to generate an authentication token").
		WithSuggestion("This is an internal server error. Please try logging in again. If the problem persists, contact support.").
		WithMetadata("retryable", true)
}

// NewSessionCreationError creates an error for session creation failures
func NewSessionCreationError(err error) *EnhancedError {
	return Wrap(err, ErrCodeSessionCreation, "Failed to create session").
		WithDetails("The system was unable to create a session").
		WithSuggestion("This is an internal server error. Please try logging in again. If the problem persists, contact support.").
		WithMetadata("retryable", true)
}

// NewNotAuthenticatedError creates an error for unauthenticated requests
func NewNotAuthenticatedError() *EnhancedError {
	return New(ErrCodeNotAuthenticated, "Authentication required").
		WithDetails("This endpoint requires authentication").
		WithSuggestion("Please log in using the /api/v1/auth/login endpoint, or include a valid API key in the 'X-API-Key' header.")
}

// NewInvalidInputError creates an error for invalid input
func NewInvalidInputError(field string, reason string) *EnhancedError {
	return New(ErrCodeInvalidInput, "Invalid input").
		WithDetails(fmt.Sprintf("Field '%s' is invalid: %s", field, reason)).
		WithSuggestion("Please check the API documentation for the expected format and try again.")
}

// NewDatabaseConnectionError creates an error for database connection failures
func NewDatabaseConnectionError(err error) *EnhancedError {
	return Wrap(err, ErrCodeDatabaseConnection, "Database connection failed").
		WithDetails("Unable to connect to the database").
		WithSuggestion("This is an internal server error. The service may be experiencing issues. Please try again in a moment.").
		WithMetadata("retryable", true)
}

// NewDatabaseQueryError creates an error for database query failures
func NewDatabaseQueryError(err error, operation string) *EnhancedError {
	return Wrap(err, ErrCodeDatabaseQuery, "Database query failed").
		WithDetails(fmt.Sprintf("Failed to execute database operation: %s", operation)).
		WithSuggestion("This is an internal server error. If the problem persists, contact support.").
		WithMetadata("retryable", true)
}
