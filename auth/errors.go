package auth

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ErrorCode represents standardized error codes for the API
type ErrorCode string

const (
	// Client errors
	ErrInvalidRequest   ErrorCode = "invalid_request"
	ErrInvalidClient    ErrorCode = "invalid_client"
	ErrInvalidGrant     ErrorCode = "invalid_grant"
	ErrInvalidScope     ErrorCode = "invalid_scope"
	ErrUnauthorized     ErrorCode = "unauthorized"
	ErrForbidden        ErrorCode = "forbidden"
	ErrNotFound         ErrorCode = "not_found"
	ErrConflict         ErrorCode = "conflict"
	ErrValidationFailed ErrorCode = "validation_failed"

	// Server errors
	ErrInternalServer     ErrorCode = "internal_server_error"
	ErrServiceUnavailable ErrorCode = "service_unavailable"
	ErrDatabaseError      ErrorCode = "database_error"
)

// APIError represents a structured API error
type APIError struct {
	Code        ErrorCode `json:"error"`
	Message     string    `json:"error_description"`
	StatusCode  int       `json:"-"`
	RequestID   string    `json:"request_id,omitempty"`
	Details     string    `json:"details,omitempty"`
	originalErr error     `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, http.StatusText(e.StatusCode), e.Message)
}

// NewAPIError creates a new API error
func NewAPIError(code ErrorCode, message string, statusCode int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WithDetails adds additional details to the error
func (e *APIError) WithDetails(details string) *APIError {
	e.Details = details
	return e
}

// WithOriginalError stores the original error for logging
func (e *APIError) WithOriginalError(err error) *APIError {
	e.originalErr = err
	return e
}

// RespondWithError sends an error response to the client and logs it
func RespondWithError(c *gin.Context, apiErr *APIError) {
	logger := GetRequestLogger(c)

	// Get request ID if available
	requestID := GetRequestID(c)
	apiErr.RequestID = requestID

	// Log the error
	logLevel := logger.Error()
	if apiErr.originalErr != nil {
		logLevel = logLevel.Err(apiErr.originalErr)
	}

	logLevel.
		Str("error_code", string(apiErr.Code)).
		Str("error_message", apiErr.Message).
		Str("details", apiErr.Details).
		Int("status_code", apiErr.StatusCode).
		Msg("API error response")

	// Return error response
	c.Header("Content-Type", "application/json")
	c.JSON(apiErr.StatusCode, apiErr)
}

// ValidateRequest validates request data and returns an error if validation fails
func ValidateRequest(c *gin.Context, validator func() error) *APIError {
	if err := validator(); err != nil {
		return NewAPIError(
			ErrValidationFailed,
			"Request validation failed",
			http.StatusBadRequest,
		).WithOriginalError(err).WithDetails(err.Error())
	}
	return nil
}

// HandleDatabaseError converts database errors to API errors
func HandleDatabaseError(err error, logger zerolog.Logger) *APIError {
	if err == nil {
		return nil
	}

	logger.Error().Err(err).Msg("Database error occurred")

	// Could add logic here to distinguish between different types of database errors
	return NewAPIError(
		ErrDatabaseError,
		"Database operation failed",
		http.StatusInternalServerError,
	).WithOriginalError(err)
}

// HandlePanicError converts panic values to API errors
func HandlePanicError(panicValue interface{}, logger zerolog.Logger) *APIError {
	logger.Error().
		Interface("panic_value", panicValue).
		Msg("Panic recovered")

	return NewAPIError(
		ErrInternalServer,
		"An unexpected error occurred",
		http.StatusInternalServerError,
	)
}

// Common error constructors

// ErrBadRequest creates a 400 Bad Request error
func ErrBadRequest(message string) *APIError {
	return NewAPIError(ErrInvalidRequest, message, http.StatusBadRequest)
}

// ErrUnauthorizedError creates a 401 Unauthorized error
func ErrUnauthorizedError(message string) *APIError {
	return NewAPIError(ErrUnauthorized, message, http.StatusUnauthorized)
}

// ErrForbiddenError creates a 403 Forbidden error
func ErrForbiddenError(message string) *APIError {
	return NewAPIError(ErrForbidden, message, http.StatusForbidden)
}

// ErrNotFoundError creates a 404 Not Found error
func ErrNotFoundError(message string) *APIError {
	return NewAPIError(ErrNotFound, message, http.StatusNotFound)
}

// ErrConflictError creates a 409 Conflict error
func ErrConflictError(message string) *APIError {
	return NewAPIError(ErrConflict, message, http.StatusConflict)
}

// ErrInternalServerError creates a 500 Internal Server Error
func ErrInternalServerError(message string) *APIError {
	return NewAPIError(ErrInternalServer, message, http.StatusInternalServerError)
}

// ErrServiceUnavailableError creates a 503 Service Unavailable error
func ErrServiceUnavailableError(message string) *APIError {
	return NewAPIError(ErrServiceUnavailable, message, http.StatusServiceUnavailable)
}
