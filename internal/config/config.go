package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configDir = ".hawkeye"
const configFile = "config.json"

type Config struct {
	Server    string `json:"server"`
	Username  string `json:"username,omitempty"`
	Token     string `json:"token,omitempty"`
	OrgUUID   string `json:"org_uuid,omitempty"`
	ProjectID string `json:"project_uuid,omitempty"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Server == "" {
		return fmt.Errorf("not logged in. Run: hawkeye login <server-url> -u <username> -p <password>")
	}
	if c.Token == "" {
		return fmt.Errorf("not authenticated. Run: hawkeye login <server-url> -u <username> -p <password>")
	}
	return nil
}

func (c *Config) ValidateProject() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ProjectID == "" {
		return fmt.Errorf("project not set. Run: hawkeye set project <uuid>")
	}
	return nil
}
