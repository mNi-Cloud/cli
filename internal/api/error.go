package api

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a failed api-gateway response. It carries the HTTP status so callers
// can tell a missing resource apart from a transport or server failure.
type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	message := e.Message
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if message == "" {
		message = "unexpected response"
	}
	return fmt.Sprintf("%s (status %d)", message, e.StatusCode)
}

// StatusCode returns the HTTP status an error carries, if it came from the API.
func StatusCode(err error) (int, bool) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode, true
	}
	return 0, false
}

// IsNotFound reports whether the server answered 404.
func IsNotFound(err error) bool {
	return hasStatus(err, http.StatusNotFound)
}

// IsUnauthorized reports whether the server rejected the credentials.
func IsUnauthorized(err error) bool {
	return hasStatus(err, http.StatusUnauthorized)
}

// IsForbidden reports whether the caller is known but not allowed.
func IsForbidden(err error) bool {
	return hasStatus(err, http.StatusForbidden)
}

func hasStatus(err error, status int) bool {
	code, ok := StatusCode(err)
	return ok && code == status
}
