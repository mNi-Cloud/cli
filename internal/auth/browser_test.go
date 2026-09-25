package auth

import (
	"errors"
	"strings"
	"testing"
)

// environment is a set of environment variables that stands in for the real one.
func environment(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestSSHWithoutDisplay(t *testing.T) {
	const connection = "192.0.2.10 50022 192.0.2.20 22"

	tests := []struct {
		name string
		vars map[string]string
		want bool
	}{
		{name: "local desktop", vars: map[string]string{"DISPLAY": ":0"}, want: false},
		{name: "local wayland desktop", vars: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, want: false},
		{name: "local console", vars: map[string]string{}, want: false},
		{name: "SSH_CONNECTION only", vars: map[string]string{"SSH_CONNECTION": connection}, want: true},
		{name: "SSH_TTY only", vars: map[string]string{"SSH_TTY": "/dev/pts/3"}, want: true},
		{name: "SSH_CONNECTION and SSH_TTY", vars: map[string]string{"SSH_CONNECTION": connection, "SSH_TTY": "/dev/pts/3"}, want: true},
		{name: "SSH with X forwarding", vars: map[string]string{"SSH_CONNECTION": connection, "SSH_TTY": "/dev/pts/3", "DISPLAY": "localhost:10.0"}, want: false},
		{name: "SSH_TTY with wayland", vars: map[string]string{"SSH_TTY": "/dev/pts/3", "WAYLAND_DISPLAY": "wayland-0"}, want: false},
		{name: "SSH_CONNECTION with wayland", vars: map[string]string{"SSH_CONNECTION": connection, "WAYLAND_DISPLAY": "wayland-0"}, want: false},
		{name: "SSH with an empty DISPLAY", vars: map[string]string{"SSH_CONNECTION": connection, "DISPLAY": ""}, want: true},
		{name: "empty SSH variables", vars: map[string]string{"SSH_CONNECTION": "", "SSH_TTY": ""}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sshWithoutDisplay(environment(tt.vars)); got != tt.want {
				t.Errorf("sshWithoutDisplay(%v) = %v, want %v", tt.vars, got, tt.want)
			}
		})
	}
}

func TestOpenBrowserRefusesAnSSHSessionWithoutADisplay(t *testing.T) {
	err := openBrowser("https://issuer.test/auth", environment(map[string]string{"SSH_TTY": "/dev/pts/3"}))

	if !errors.Is(err, errSSHWithoutDisplay) {
		t.Fatalf("openBrowser() error = %v, want %v", err, errSSHWithoutDisplay)
	}
	if !strings.Contains(err.Error(), "SSH") {
		t.Errorf("openBrowser() error = %q, want it to say the session is over SSH", err)
	}
}
