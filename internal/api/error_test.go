package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "with message",
			err:  &Error{StatusCode: http.StatusNotFound, Message: "vpcs \"a\" not found"},
			want: "vpcs \"a\" not found (status 404)",
		},
		{
			name: "without message",
			err:  &Error{StatusCode: http.StatusInternalServerError},
			want: "Internal Server Error (status 500)",
		},
		{
			name: "unknown status without message",
			err:  &Error{StatusCode: 599},
			want: "unexpected response (status 599)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not found", err: &Error{StatusCode: http.StatusNotFound}, want: true},
		{name: "wrapped not found", err: fmt.Errorf("get: %w", &Error{StatusCode: http.StatusNotFound}), want: true},
		{name: "forbidden", err: &Error{StatusCode: http.StatusForbidden}, want: false},
		{name: "transport failure", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !IsUnauthorized(&Error{StatusCode: http.StatusUnauthorized}) {
		t.Error("IsUnauthorized(401) = false, want true")
	}
	if IsUnauthorized(errors.New("boom")) {
		t.Error("IsUnauthorized(plain error) = true, want false")
	}
}

func TestIsForbidden(t *testing.T) {
	if !IsForbidden(&Error{StatusCode: http.StatusForbidden}) {
		t.Error("IsForbidden(403) = false, want true")
	}
	if IsForbidden(&Error{StatusCode: http.StatusNotFound}) {
		t.Error("IsForbidden(404) = true, want false")
	}
}

func TestStatusCode(t *testing.T) {
	code, ok := StatusCode(fmt.Errorf("wrapped: %w", &Error{StatusCode: http.StatusConflict}))
	if !ok {
		t.Fatal("StatusCode() reported no status for an api error")
	}
	if code != http.StatusConflict {
		t.Errorf("StatusCode() = %d, want %d", code, http.StatusConflict)
	}

	if _, ok := StatusCode(errors.New("boom")); ok {
		t.Error("StatusCode() reported a status for a plain error")
	}
}
