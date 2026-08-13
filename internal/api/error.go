package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	bearerScheme = "Bearer"

	// insufficientScope is the code RFC 6750 §3.1 gives a token that is good but
	// was not granted what the request needs.
	insufficientScope = "insufficient_scope"
)

// Error is a failed api-gateway response. It carries the HTTP status so callers
// can tell a missing resource apart from a transport or server failure, and the
// WWW-Authenticate header as Challenge, which says what the token was refused
// for.
type Error struct {
	StatusCode int
	Message    string
	Challenge  string
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

// IsInsufficientScope reports whether the server refused the token because it
// was not granted what the request needs. RFC 6749 §6 does not let a refresh
// widen the scopes of a token, so only a new login can lift this.
func IsInsufficientScope(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return bearerParams(apiErr.Challenge)["error"] == insufficientScope
}

func hasStatus(err error, status int) bool {
	code, ok := StatusCode(err)
	return ok && code == status
}

// bearerParams reads the parameters of a Bearer challenge, as RFC 6750 §3
// writes them: comma separated pairs whose value may be quoted.
func bearerParams(challenge string) map[string]string {
	rest := strings.TrimSpace(challenge)
	if len(rest) < len(bearerScheme) || !strings.EqualFold(rest[:len(bearerScheme)], bearerScheme) {
		return nil
	}

	params := map[string]string{}
	for _, pair := range splitParams(strings.TrimSpace(rest[len(bearerScheme):])) {
		name, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(name))] = unquote(strings.TrimSpace(value))
	}
	return params
}

// splitParams cuts a challenge on the commas that stand between parameters and
// leaves the ones inside a quoted value alone.
func splitParams(list string) []string {
	params := []string{}
	quoted, start := false, 0

	for i := range len(list) {
		switch list[i] {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				params = append(params, list[start:i])
				start = i + 1
			}
		}
	}
	return append(params, list[start:])
}

func unquote(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
