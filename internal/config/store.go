package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

const (
	dirPerm         = 0o700
	configPerm      = 0o644
	credentialsPerm = 0o600
)

// Store reads and writes the profile files.
type Store struct {
	configPath      string
	credentialsPath string
}

// NewStore builds a store over the files this machine uses by default.
func NewStore() (*Store, error) {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return NewStoreAt(configPath, DefaultCredentialsPath(configPath)), nil
}

// NewStoreAt builds a store over explicit paths.
func NewStoreAt(configPath, credentialsPath string) *Store {
	return &Store{configPath: configPath, credentialsPath: credentialsPath}
}

// ConfigPath returns the file the contexts are read from.
func (s *Store) ConfigPath() string { return s.configPath }

// CredentialsPath returns the file the tokens are read from.
func (s *Store) CredentialsPath() string { return s.credentialsPath }

// LoadConfig reads the contexts. A file that does not exist yet is an empty
// profile, which is what `mni login` starts from.
func (s *Store) LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := readYAML(s.configPath, cfg); err != nil {
		return nil, fmt.Errorf("cannot read config %s: %w", s.configPath, err)
	}
	return cfg, nil
}

// SaveConfig writes the contexts back.
func (s *Store) SaveConfig(cfg *Config) error {
	if err := writeYAML(s.configPath, cfg, configPerm); err != nil {
		return fmt.Errorf("cannot write config %s: %w", s.configPath, err)
	}
	return nil
}

// LoadCredentials reads the tokens. A file that does not exist yet means
// nobody has logged in on this machine.
func (s *Store) LoadCredentials() (*Credentials, error) {
	creds := &Credentials{}
	if err := readYAML(s.credentialsPath, creds); err != nil {
		return nil, fmt.Errorf("cannot read credentials %s: %w", s.credentialsPath, err)
	}
	return creds, nil
}

// SaveCredentials writes the tokens back.
func (s *Store) SaveCredentials(creds *Credentials) error {
	if err := writeYAML(s.credentialsPath, creds, credentialsPerm); err != nil {
		return fmt.Errorf("cannot write credentials %s: %w", s.credentialsPath, err)
	}
	return nil
}

// Credential returns the stored token set of one context.
func (s *Store) Credential(contextName string) (Credential, bool, error) {
	creds, err := s.LoadCredentials()
	if err != nil {
		return Credential{}, false, err
	}
	cred, found := creds.Find(contextName)
	return cred, found, nil
}

// SaveCredential stores the token set of one context, leaving the others alone.
func (s *Store) SaveCredential(cred Credential) error {
	creds, err := s.LoadCredentials()
	if err != nil {
		return err
	}
	creds.Put(cred)
	return s.SaveCredentials(creds)
}

// DeleteCredential drops the token set of one context and reports whether it
// was there.
func (s *Store) DeleteCredential(contextName string) (bool, error) {
	creds, err := s.LoadCredentials()
	if err != nil {
		return false, err
	}
	if !creds.Remove(contextName) {
		return false, nil
	}
	if err := s.SaveCredentials(creds); err != nil {
		return false, err
	}
	return true, nil
}

func readYAML(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	return yaml.Unmarshal(raw, into)
}

// writeYAML replaces a file through a temporary one in the same directory, so
// that a crash mid-write cannot leave a half-written profile behind.
func writeYAML(path string, from any, perm os.FileMode) error {
	raw, err := yaml.Marshal(from)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
