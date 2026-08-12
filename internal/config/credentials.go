package config

import "time"

// Credentials is the whole token file.
type Credentials struct {
	Credentials []Credential `yaml:"credentials"`
}

// Credential is the token set of one context.
type Credential struct {
	Context      string    `yaml:"context"`
	AccessToken  string    `yaml:"accessToken"`
	RefreshToken string    `yaml:"refreshToken,omitempty"`
	ExpiresAt    time.Time `yaml:"expiresAt"`
}

// Expired reports whether the access token is past its lifetime, counting a
// leeway so that a token cannot die while a request is in flight.
func (c Credential) Expired(now time.Time, leeway time.Duration) bool {
	return !c.ExpiresAt.After(now.Add(leeway))
}

// Find returns the credential of a context.
func (c *Credentials) Find(contextName string) (Credential, bool) {
	for _, cred := range c.Credentials {
		if cred.Context == contextName {
			return cred, true
		}
	}
	return Credential{}, false
}

// Put adds a credential or replaces the one of the same context.
func (c *Credentials) Put(cred Credential) {
	for i := range c.Credentials {
		if c.Credentials[i].Context == cred.Context {
			c.Credentials[i] = cred
			return
		}
	}
	c.Credentials = append(c.Credentials, cred)
}

// Remove drops the credential of a context and reports whether it was there.
func (c *Credentials) Remove(contextName string) bool {
	for i := range c.Credentials {
		if c.Credentials[i].Context != contextName {
			continue
		}
		c.Credentials = append(c.Credentials[:i], c.Credentials[i+1:]...)
		return true
	}
	return false
}
