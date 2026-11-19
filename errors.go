package veilroute

import (
	"errors"
	"fmt"
)

// Sentinel errors, matched with errors.Is against an *APIError:
//
//	if errors.Is(err, veilroute.ErrInsufficientBalance) { /* top up */ }
var (
	ErrMissingAPIKey       = errors.New("veilroute: missing API key")
	ErrBadRequest          = errors.New("veilroute: bad request")          // 400
	ErrUnauthorized        = errors.New("veilroute: unauthorized")         // 401
	ErrInsufficientBalance = errors.New("veilroute: insufficient balance") // 402
	ErrPermissionDenied    = errors.New("veilroute: permission denied")    // 403
	ErrNotFound            = errors.New("veilroute: not found")            // 404
	ErrTimeout             = errors.New("veilroute: request timeout")      // 408
	ErrUnsupportedTarget   = errors.New("veilroute: unsupported target")   // 422
	ErrRateLimited         = errors.New("veilroute: rate limited")         // 429
	ErrServer              = errors.New("veilroute: server error")         // 5xx
	ErrBadGateway          = errors.New("veilroute: bad gateway")          // 502
	ErrServiceUnavailable  = errors.New("veilroute: service unavailable")  // 503
)

// APIError is returned when the API answers with an error status. Messages are
// intentionally generic, so branch on StatusCode or errors.Is, not Message.
type APIError struct {
	StatusCode int
	Message    string
	Code       string
	Param      string
	Retryable  bool
	RequestID  string
	Body       map[string]any
}

func (e *APIError) Error() string {
	return fmt.Sprintf("veilroute: %d: %s", e.StatusCode, e.Message)
}

// Is lets errors.Is match the sentinel for the status code.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == 400
	case ErrUnauthorized:
		return e.StatusCode == 401
	case ErrInsufficientBalance:
		return e.StatusCode == 402
	case ErrPermissionDenied:
		return e.StatusCode == 403
	case ErrNotFound:
		return e.StatusCode == 404
	case ErrTimeout:
		return e.StatusCode == 408
	case ErrUnsupportedTarget:
		return e.StatusCode == 422
	case ErrRateLimited:
		return e.StatusCode == 429
	case ErrBadGateway:
		return e.StatusCode == 502
	case ErrServiceUnavailable:
		return e.StatusCode == 503
	case ErrServer:
		return e.StatusCode >= 500
	}
	return false
}

func newAPIError(status int, body map[string]any) *APIError {
	msg := "request failed"
	for _, k := range []string{"error", "err", "message"} {
		if s, ok := body[k].(string); ok && s != "" {
			msg = s
			break
		}
	}
	out := &APIError{StatusCode: status, Message: msg, Body: body}
	if s, ok := body["code"].(string); ok {
		out.Code = s
	}
	if s, ok := body["param"].(string); ok {
		out.Param = s
	}
	if b, ok := body["retryable"].(bool); ok {
		out.Retryable = b
	}
	if s, ok := body["request_id"].(string); ok {
		out.RequestID = s
	}
	return out
}

// ConnectionError is returned when no response was received (network error,
// timeout or cancelled context).
type ConnectionError struct{ Err error }

func (e *ConnectionError) Error() string { return "veilroute: connection error: " + e.Err.Error() }
func (e *ConnectionError) Unwrap() error { return e.Err }
