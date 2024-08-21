package pkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/oauth2"
)

func Generate() (Code, error) { return generate(rand.Reader) }

func generate(rand io.Reader) (Code, error) {
	var buf [32]byte
	if _, err := io.ReadFull(rand, buf[:]); err != nil {
		return "", fmt.Errorf("could not generate PKCE code: %w", err)
	}
	return Code(hex.EncodeToString(buf[:])), nil
}

type Code string

func (p *Code) Challenge() oauth2.AuthCodeOption {
	b := sha256.Sum256([]byte(*p))
	return oauth2.SetAuthURLParam("code_challenge", base64.RawURLEncoding.EncodeToString(b[:]))
}

func (p *Code) Method() oauth2.AuthCodeOption {
	return oauth2.SetAuthURLParam("code_challenge_method", "S256")
}

func (p *Code) Verifier() oauth2.AuthCodeOption {
	return oauth2.SetAuthURLParam("code_verifier", string(*p))
}
