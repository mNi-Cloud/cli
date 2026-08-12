package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv(configPathEnv, "")
	t.Setenv(xdgConfigHomeEnv, "/xdg")

	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error = %v", err)
	}

	want := filepath.Join("/xdg", appDirName, configFileName)
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathFallsBackToHome(t *testing.T) {
	t.Setenv(configPathEnv, "")
	t.Setenv(xdgConfigHomeEnv, "")
	t.Setenv("HOME", "/home/tester")

	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error = %v", err)
	}

	want := filepath.Join("/home/tester", ".config", appDirName, configFileName)
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathHonorsOverride(t *testing.T) {
	t.Setenv(xdgConfigHomeEnv, "/xdg")
	t.Setenv(configPathEnv, "/somewhere/mine.yaml")

	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() error = %v", err)
	}

	if got != "/somewhere/mine.yaml" {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, "/somewhere/mine.yaml")
	}
}

func TestDefaultCredentialsPathSitsNextToConfig(t *testing.T) {
	t.Setenv(credentialsPathEnv, "")

	got := DefaultCredentialsPath("/somewhere/mine.yaml")
	want := filepath.Join("/somewhere", credentialsFileName)
	if got != want {
		t.Errorf("DefaultCredentialsPath() = %q, want %q", got, want)
	}
}

func TestDefaultCredentialsPathHonorsOverride(t *testing.T) {
	t.Setenv(credentialsPathEnv, "/vault/tokens.yaml")

	if got := DefaultCredentialsPath("/somewhere/mine.yaml"); got != "/vault/tokens.yaml" {
		t.Errorf("DefaultCredentialsPath() = %q, want %q", got, "/vault/tokens.yaml")
	}
}
