package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

const (
	testPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICAsmP6hTDklJA+6RvDFs7ybEaLVLxUc5UXW8bACNZ6C user@example.com"

	testPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACAgLJj+oUw5JSQPukbwxbO8mxGi1S8VHOVF1vGwAjWegg==
-----END OPENSSH PRIVATE KEY-----
`
)

func writeKeyFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

// answerCreatedSSHKey lets the fake gateway answer a create the way the server
// does, with the key it stored and the fingerprint it worked out.
func answerCreatedSSHKey(env *testEnv, name string) {
	env.object[sshKeyPath] = unstructured.Unstructured{
		"apiVersion": "vm/v1alpha1",
		"kind":       "SSHKey",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"publicKey": testPublicKey},
		"status": map[string]any{
			"publicKey":   testPublicKey,
			"fingerprint": "SHA256:0Fs2kWvNz3s2Bx1H3JZ0e0Yq3S3sRk4Q0v0h6Zq2Cw0",
			"phase":       "Pending",
		},
	}
}

func TestSSHKeyAddSendsTheKeyTheFileHolds(t *testing.T) {
	env := loggedIn(t)
	answerCreatedSSHKey(env, "clidbg-key")
	path := writeKeyFile(t, "id_ed25519.pub", testPublicKey+"\n")

	out, err := env.run(t, "ssh-key", "add", "clidbg-key", path)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !env.sent(http.MethodPost, sshKeyPath) {
		t.Fatalf("no POST reached %q, the last request went to %q", sshKeyPath, env.lastPath())
	}

	body := env.bodyOf(http.MethodPost, sshKeyPath)
	want := []string{
		`"apiVersion":"vm/v1alpha1"`,
		`"kind":"SSHKey"`,
		`"name":"clidbg-key"`,
		`"publicKey":"` + testPublicKey + `"`,
	}
	for _, hold := range want {
		if !strings.Contains(body, hold) {
			t.Errorf("body = %q, want it to hold %s", body, hold)
		}
	}

	if !strings.Contains(out, "sshkeys/clidbg-key created") {
		t.Errorf("ssh-key add = %q, want it to report the key", out)
	}
}

func TestSSHKeyAddTakesAFileWithSpaceAroundTheKey(t *testing.T) {
	env := loggedIn(t)
	answerCreatedSSHKey(env, "clidbg-key")
	path := writeKeyFile(t, "id_ed25519.pub", " "+testPublicKey+" \n\n")

	if _, err := env.run(t, "ssh-key", "add", "clidbg-key", path); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	body := env.bodyOf(http.MethodPost, sshKeyPath)
	if !strings.Contains(body, `"publicKey":"`+testPublicKey+`"`) {
		t.Errorf("body = %q, want the key without the space around it", body)
	}
}

func TestSSHKeyAddRefusesAFileThatIsNoPublicKey(t *testing.T) {
	env := loggedIn(t)
	path := writeKeyFile(t, "id_ed25519.pub", "not a key at all\n")

	_, err := env.run(t, "ssh-key", "add", "clidbg-key", path)
	if err == nil {
		t.Fatal("run() error = nil, want a file that is no public key refused")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("run() error = %q, want it to name the file", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the file is no public key", len(env.requests))
	}
}

func TestSSHKeyAddSaysWhenTheFileIsAPrivateKey(t *testing.T) {
	env := loggedIn(t)
	path := writeKeyFile(t, "id_ed25519", testPrivateKey)

	_, err := env.run(t, "ssh-key", "add", "clidbg-key", path)
	if err == nil {
		t.Fatal("run() error = nil, want a private key refused")
	}
	if !strings.Contains(err.Error(), "private key") {
		t.Errorf("run() error = %q, want it to say what the file is", err)
	}
	if !strings.Contains(err.Error(), ".pub") {
		t.Errorf("run() error = %q, want it to name the file to give instead", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the file is a private key", len(env.requests))
	}
}

func TestSSHKeyAddReportsAFileThatIsNotThere(t *testing.T) {
	env := loggedIn(t)
	path := filepath.Join(t.TempDir(), "id_ed25519.pub")

	_, err := env.run(t, "ssh-key", "add", "clidbg-key", path)
	if err == nil {
		t.Fatal("run() error = nil, want a file that is not there reported")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("run() error = %q, want it to name the file", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although there is no file to read", len(env.requests))
	}
}

func TestSSHKeyAddNeedsANameAndAFile(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ssh-key", "add"); err == nil {
		t.Fatal("run() error = nil, want a name asked for")
	}
	if _, err := env.run(t, "ssh-key", "add", "clidbg-key"); err == nil {
		t.Fatal("run() error = nil, want a public key file asked for")
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the command is not whole", len(env.requests))
	}
}

func TestSSHKeyAddReportsWhatTheServerAnswered(t *testing.T) {
	env := loggedIn(t)
	env.failures[sshKeyPath] = api.Response[any]{Message: `an SSH key named "clidbg-key" is already there`}
	path := writeKeyFile(t, "id_ed25519.pub", testPublicKey+"\n")

	_, err := env.run(t, "ssh-key", "add", "clidbg-key", path)
	if err == nil {
		t.Fatal("run() error = nil, want the refusal of the server reported")
	}
	if !strings.Contains(UserFacing(err).Error(), "is already there") {
		t.Errorf("run() error = %q, want the message of the server", err)
	}
}

// TestSSHKeyOffersOnlyAdd keeps `mni get sshkeys` and `mni delete sshkey` the
// one way to read and to remove a key, so that the same thing is not asked for
// in two ways.
func TestSSHKeyOffersOnlyAdd(t *testing.T) {
	root := NewCommandFor("test", NewDeps(nil, io.Discard, io.Discard))

	command := root.Command("ssh-key")
	if command == nil {
		t.Fatal("mni offers no ssh-key command")
	}

	offered := map[string]bool{}
	for _, sub := range command.Commands {
		offered[sub.Name] = true
	}
	if !offered["add"] {
		t.Error("ssh-key offers no add")
	}
	for _, name := range []string{"list", "get", "delete", "remove"} {
		if offered[name] {
			t.Errorf("ssh-key offers %q, want the generic commands to stay the one way to do it", name)
		}
	}
}
