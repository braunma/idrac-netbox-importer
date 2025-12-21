// Package errors provides custom error types and error handling utilities
// for the iDRAC inventory application.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure conditions.
var (
	// ErrConnectionFailed indicates a network connection failure.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrAuthenticationFailed indicates invalid credentials.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrTimeout indicates a request timeout.
	ErrTimeout = errors.New("request timed out")

	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidResponse indicates an unexpected API response.
	ErrInvalidResponse = errors.New("invalid response from server")

	// ErrConfigInvalid indicates invalid configuration.
	ErrConfigInvalid = errors.New("invalid configuration")

	// ErrNoServers indicates no servers are configured.
	ErrNoServers = errors.New("no servers configured")
)

// RedfishError represents an error returned by the Redfish API.
type RedfishError struct {
	StatusCode int
	Status     string
	Message    string
	Host       string
	Path       string
}

func (e *RedfishError) Error() string {
	return fmt.Sprintf("redfish error on %s%s: %s (HTTP %d)", e.Host, e.Path, e.Message, e.StatusCode)
}

// IsAuthError returns true if this is an authentication error.
func (e *RedfishError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}

// IsNotFound returns true if this is a not found error.
func (e *RedfishError) IsNotFound() bool {
	return e.StatusCode == 404
}

// NewRedfishError creates a new RedfishError.
func NewRedfishError(host, path string, statusCode int, status, message string) *RedfishError {
	return &RedfishError{
		Host:       host,
		Path:       path,
		StatusCode: statusCode,
		Status:     status,
		Message:    message,
	}
}

// CollectionError represents an error that occurred during hardware collection.
type CollectionError struct {
	Host      string
	Component string
	Err       error
}

func (e *CollectionError) Error() string {
	return fmt.Sprintf("failed to collect %s from %s: %v", e.Component, e.Host, e.Err)
}

func (e *CollectionError) Unwrap() error {
	return e.Err
}

// NewCollectionError creates a new CollectionError.
func NewCollectionError(host, component string, err error) *CollectionError {
	return &CollectionError{
		Host:      host,
		Component: component,
		Err:       err,
	}
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error in %s: %s", e.Field, e.Message)
}

// NewConfigError creates a new ConfigError.
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Message: message,
	}
}

// MultiError aggregates multiple errors.
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred; first: %v", len(e.Errors), e.Errors[0])
}

// Add appends an error to the MultiError.
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if any errors were collected.
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ErrorOrNil returns nil if no errors, otherwise returns the MultiError.
func (e *MultiError) ErrorOrNil() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

// Is implements errors.Is for MultiError.
func (e *MultiError) Is(target error) bool {
	for _, err := range e.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
