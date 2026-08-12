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

func TestIsInsufficientScope(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "insufficient scope",
			err:  &Error{StatusCode: http.StatusForbidden, Challenge: `Bearer error="insufficient_scope", scope="mni:api"`},
			want: true,
		},
		{
			name: "a challenge with a realm before the error",
			err:  &Error{StatusCode: http.StatusForbidden, Challenge: `Bearer realm="mni", error="insufficient_scope", error_description="the token may not call this API", scope="mni:api"`},
			want: true,
		},
		{
			name: "a comma inside a value",
			err:  &Error{StatusCode: http.StatusForbidden, Challenge: `Bearer error_description="this token, issued elsewhere, may not call the API", error="insufficient_scope"`},
			want: true,
		},
		{
			name: "an unquoted value",
			err:  &Error{StatusCode: http.StatusForbidden, Challenge: "Bearer error=insufficient_scope"},
			want: true,
		},
		{
			name: "a lower case scheme",
			err:  &Error{StatusCode: http.StatusForbidden, Challenge: `bearer error="insufficient_scope"`},
			want: true,
		},
		{
			name: "wrapped",
			err:  fmt.Errorf("list: %w", &Error{StatusCode: http.StatusForbidden, Challenge: `Bearer error="insufficient_scope"`}),
			want: true,
		},
		{
			name: "another bearer error",
			err:  &Error{StatusCode: http.StatusUnauthorized, Challenge: `Bearer error="invalid_token"`},
			want: false,
		},
		{
			name: "a challenge without an error",
			err:  &Error{StatusCode: http.StatusUnauthorized, Challenge: "Bearer"},
			want: false,
		},
		{
			name: "another scheme",
			err:  &Error{StatusCode: http.StatusUnauthorized, Challenge: `Basic realm="insufficient_scope"`},
			want: false,
		},
		{
			name: "forbidden without a challenge",
			err:  &Error{StatusCode: http.StatusForbidden, Message: "Forbidden"},
			want: false,
		},
		{name: "plain error", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInsufficientScope(tt.err); got != tt.want {
				t.Errorf("IsInsufficientScope(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
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
