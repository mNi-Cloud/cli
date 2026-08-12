package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	configPathEnv      = "MNI_CONFIG"
	credentialsPathEnv = "MNI_CREDENTIALS"
	xdgConfigHomeEnv   = "XDG_CONFIG_HOME"

	appDirName          = "mni"
	configFileName      = "config.yaml"
	credentialsFileName = "credentials.yaml"
)

// DefaultConfigPath returns the config file this machine reads, honoring the
// direct override first and the XDG base directory spec after it.
func DefaultConfigPath() (string, error) {
	if override := os.Getenv(configPathEnv); override != "" {
		return override, nil
	}

	if base := os.Getenv(xdgConfigHomeEnv); base != "" {
		return filepath.Join(base, appDirName, configFileName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate the home directory: %w", err)
	}
	return filepath.Join(home, ".config", appDirName, configFileName), nil
}

// DefaultCredentialsPath returns the token file that belongs to a config file.
// Tokens sit next to the config they belong to so that pointing MNI_CONFIG at
// another file moves both halves of the profile together.
func DefaultCredentialsPath(configPath string) string {
	if override := os.Getenv(credentialsPathEnv); override != "" {
		return override
	}
	return filepath.Join(filepath.Dir(configPath), credentialsFileName)
}
