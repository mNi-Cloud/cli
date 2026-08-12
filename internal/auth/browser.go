package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser asks the desktop to open a URL.
func OpenBrowser(target string) error {
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
