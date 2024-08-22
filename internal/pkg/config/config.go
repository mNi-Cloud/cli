package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type (
	Config struct {
		Token              string    `json:"token"`
		RefreshToken       string    `json:"refresh_token"`
		TokenExpiry        time.Time `json:"token_expiry"`
		RefreshTokenExpiry time.Time `json:"refresh_token_expiry"`
	}
)

func SaveConfig(c *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	mniDir := filepath.Join(homeDir, ".mni")
	if _, err := os.Stat(mniDir); os.IsNotExist(err) {
		err := os.Mkdir(mniDir, 0755)
		if err != nil {
			return err
		}
	}

	configPath := filepath.Join(mniDir, "config")

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	err = os.WriteFile(configPath, data, 0700)
	if err != nil {
		return err
	}
	return nil
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	mniDir := filepath.Join(homeDir, ".mni")
	if _, err := os.Stat(mniDir); os.IsNotExist(err) {
		err := os.Mkdir(mniDir, 0755)
		if err != nil {
			return nil, err
		}
	}

	configPath := filepath.Join(mniDir, "config")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	} else if err != nil {
		return nil, err
	} else {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		} else {
			var v Config
			err = json.Unmarshal(content, &v)
			if err != nil {
				return nil, err
			}
			return &v, nil
		}
	}
}
