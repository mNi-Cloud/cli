// Package sshkey reads the public keys of this machine, in the format OpenSSH
// writes them.
package sshkey

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

var (
	errEmpty = errors.New("the file is empty")
	// A user who gives a private key meant the file beside it, so this is worth
	// a message of its own rather than the one about a line that does not parse.
	errPrivateKey  = errors.New("this is a private key, and only the public key belongs on a server: give the file whose name ends in .pub")
	errSeveralKeys = errors.New("this holds more than one line, and a public key is a single line")
)

// ReadPublicKey reads the one public key a file holds, without the space
// around it.
func ReadPublicKey(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read the public key file %s: %w", path, err)
	}

	key, err := parse(string(content))
	if err != nil {
		return "", fmt.Errorf("cannot read a public key out of %s: %w", path, err)
	}
	return key, nil
}

// parse reads the one public key a file holds. The line is returned the way it
// was written, so that the owner OpenSSH writes at the end of a key stays with
// it.
func parse(content string) (string, error) {
	key := strings.TrimSpace(content)
	switch {
	case key == "":
		return "", errEmpty
	case isPrivateKey(key):
		return "", errPrivateKey
	case strings.ContainsAny(key, "\n\r"):
		return "", errSeveralKeys
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key)); err != nil {
		return "", fmt.Errorf("this is no OpenSSH public key: %w", err)
	}
	return key, nil
}

// isPrivateKey reports whether a file holds a private key. Every format of one
// opens with a PEM line that says so.
func isPrivateKey(content string) bool {
	first, _, _ := strings.Cut(content, "\n")
	return strings.HasPrefix(first, "-----BEGIN") && strings.Contains(first, "PRIVATE KEY")
}
