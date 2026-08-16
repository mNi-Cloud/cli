package sshkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	publicKey            = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICAsmP6hTDklJA+6RvDFs7ybEaLVLxUc5UXW8bACNZ6C user@example.com"
	publicKeyWithNoOwner = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICAsmP6hTDklJA+6RvDFs7ybEaLVLxUc5UXW8bACNZ6C"

	privateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
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

func TestReadPublicKeyTakesTheKeyOutOfTheFile(t *testing.T) {
	path := writeKeyFile(t, "id_ed25519.pub", publicKey+"\n")

	key, err := ReadPublicKey(path)
	if err != nil {
		t.Fatalf("ReadPublicKey() error = %v", err)
	}
	if key != publicKey {
		t.Errorf("ReadPublicKey() = %q, want %q", key, publicKey)
	}
}

func TestReadPublicKeyNamesTheFileItCouldNotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id_ed25519.pub")

	_, err := ReadPublicKey(path)
	if err == nil {
		t.Fatal("ReadPublicKey() error = nil, want a file that is not there reported")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("ReadPublicKey() error = %q, want it to name the file", err)
	}
}

func TestReadPublicKeyNamesTheFileThatHoldsNoKey(t *testing.T) {
	path := writeKeyFile(t, "id_ed25519.pub", "not a key at all\n")

	_, err := ReadPublicKey(path)
	if err == nil {
		t.Fatal("ReadPublicKey() error = nil, want a file that holds no key refused")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("ReadPublicKey() error = %q, want it to name the file", err)
	}
}

func TestParseTakesTheKeyWithoutTheSpaceAroundIt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "as written", content: publicKey, want: publicKey},
		{name: "with a newline at the end", content: publicKey + "\n", want: publicKey},
		{name: "with space around it", content: " \t" + publicKey + " \n\n", want: publicKey},
		{name: "without an owner in it", content: publicKeyWithNoOwner + "\n", want: publicKeyWithNoOwner},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, err := parse(test.content)
			if err != nil {
				t.Fatalf("parse() error = %v", err)
			}
			if key != test.want {
				t.Errorf("parse() = %q, want %q", key, test.want)
			}
		})
	}
}

func TestParseRefusesWhatIsNoPublicKey(t *testing.T) {
	tests := []struct {
		name    string
		content string
		says    string
	}{
		{name: "nothing", content: " \n", says: "empty"},
		{name: "a private key", content: privateKey, says: ".pub"},
		{name: "words that are no key", content: "not a key at all\n", says: "no OpenSSH public key"},
		{name: "a key without its data", content: "ssh-ed25519\n", says: "no OpenSSH public key"},
		{name: "several keys", content: publicKey + "\n" + publicKeyWithNoOwner + "\n", says: "single line"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parse(test.content)
			if err == nil {
				t.Fatal("parse() error = nil, want the file refused")
			}
			if !strings.Contains(err.Error(), test.says) {
				t.Errorf("parse() error = %q, want it to say %q", err, test.says)
			}
		})
	}
}
