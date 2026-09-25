package auth

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// errSSHWithoutDisplay stops a browser from being opened in an SSH session that
// has no display to show it on.
var errSSHWithoutDisplay = errors.New("the SSH session has no display")

// OpenBrowser asks the desktop to open a URL.
func OpenBrowser(target string) error {
	return openBrowser(target, os.Getenv)
}

func openBrowser(target string, getenv func(key string) string) error {
	if sshWithoutDisplay(getenv) {
		return errSSHWithoutDisplay
	}

	name, args := browserCommand(target)
	if name == "" {
		return fmt.Errorf("no way to open a browser on %s", runtime.GOOS)
	}

	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// sshWithoutDisplay reports whether the CLI runs in an SSH session that has no
// display. There xdg-open falls back to a text browser such as w3m, which
// fights the CLI for the terminal, and open on macOS shows the page on a
// screen nobody is looking at.
func sshWithoutDisplay(getenv func(key string) string) bool {
	overSSH := getenv("SSH_CONNECTION") != "" || getenv("SSH_TTY") != ""
	hasDisplay := getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
	return overSSH && !hasDisplay
}

func browserCommand(target string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{target}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", target}
	case "linux", "freebsd", "netbsd", "openbsd":
		return "xdg-open", []string{target}
	default:
		return "", nil
	}
}
