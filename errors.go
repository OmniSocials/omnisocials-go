package omnisocials

import (
	"fmt"
	"time"
)

// Error hierarchy for the OmniSocials SDK. Match errors with errors.As:
//
//	APIError                  any non-2xx HTTP response (Status, Code, Message, Body)
//	  AuthenticationError     401
//	  PermissionDeniedError   403
//	  NotFoundError           404
//	  ValidationError         400 / 422
//	  RateLimitError          429 (exposes RetryAfter)
//	  ServerError             >= 500
//	ConnectionError           network failure / timeout
//	WebhookVerificationError  invalid webhook signature
//
// Every typed wrapper unwraps to its embedded *APIError, so both of these work:
//
//	var nf *omnisocials.NotFoundError
//	if errors.As(err, &nf) { ... }
//
//	var apiErr *omnisocials.APIError
//	if errors.As(err, &apiErr) { fmt.Println(apiErr.Status, apiErr.Code) }

// APIError is returned for any non-2xx HTTP response. Code and Message come
// from the API error body `{ "error": { "code", "message" } }` when present.
type APIError struct {
	// Status is the HTTP status code of the failed response.
	Status int
	// Code is the machine-readable error code from the API body
	// (e.g. "validation_error", "insufficient_scope").
	Code string
	// Message is the human-readable error message.
	Message string
	// Body is the parsed JSON response body, when the API returned JSON.
	Body any
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("omnisocials: API error %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("omnisocials: API error %d: %s", e.Status, e.Message)
}

// AuthenticationError is returned for 401 responses, and from NewClient when
// no API key is configured.
type AuthenticationError struct{ APIError }

// Unwrap lets errors.As also match the embedded *APIError.
func (e *AuthenticationError) Unwrap() error { return &e.APIError }

// PermissionDeniedError is returned for 403 responses (e.g. a missing scope).
type PermissionDeniedError struct{ APIError }

// Unwrap lets errors.As also match the embedded *APIError.
func (e *PermissionDeniedError) Unwrap() error { return &e.APIError }

// NotFoundError is returned for 404 responses.
type NotFoundError struct{ APIError }

// Unwrap lets errors.As also match the embedded *APIError.
func (e *NotFoundError) Unwrap() error { return &e.APIError }

// ValidationError is returned for 400 and 422 responses.
type ValidationError struct{ APIError }

// Unwrap lets errors.As also match the embedded *APIError.
func (e *ValidationError) Unwrap() error { return &e.APIError }

// RateLimitError is returned for 429 responses once automatic retries are
// exhausted.
type RateLimitError struct {
	APIError
	// RetryAfter is the wait parsed from the Retry-After header;
	// zero when the header was absent.
	RetryAfter time.Duration
}

// Unwrap lets errors.As also match the embedded *APIError.
func (e *RateLimitError) Unwrap() error { return &e.APIError }

// ServerError is returned for responses with status >= 500 once automatic
// retries are exhausted.
type ServerError struct{ APIError }

// Unwrap lets errors.As also match the embedded *APIError.
func (e *ServerError) Unwrap() error { return &e.APIError }

// ConnectionError is returned on network failures and timeouts (after
// automatic retries are exhausted).
type ConnectionError struct {
	Message string
	// Err is the underlying transport error, if any.
	Err error
}

func (e *ConnectionError) Error() string { return "omnisocials: " + e.Message }

// Unwrap returns the underlying transport error.
func (e *ConnectionError) Unwrap() error { return e.Err }

// WebhookVerificationError is returned by VerifyWebhookSignature on any
// verification failure.
type WebhookVerificationError struct {
	Message string
}

func (e *WebhookVerificationError) Error() string { return "omnisocials: " + e.Message }

// newAPIError maps an HTTP status to the most specific APIError wrapper.
func newAPIError(status int, code, message string, body any, retryAfter time.Duration) error {
	base := APIError{Status: status, Code: code, Message: message, Body: body}
	switch {
	case status == 400 || status == 422:
		return &ValidationError{APIError: base}
	case status == 401:
		return &AuthenticationError{APIError: base}
	case status == 403:
		return &PermissionDeniedError{APIError: base}
	case status == 404:
		return &NotFoundError{APIError: base}
	case status == 429:
		return &RateLimitError{APIError: base, RetryAfter: retryAfter}
	case status >= 500:
		return &ServerError{APIError: base}
	default:
		return &base
	}
}
