package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configDir = ".hawkeye"
const configFile = "config.json"

type Config struct {
	Server    string `json:"server"`
	Username  string `json:"username,omitempty"`
	Token     string `json:"token,omitempty"`
	OrgUUID   string `json:"org_uuid,omitempty"`
	ProjectID string `json:"project_uuid,omitempty"`
	Profile   string `json:"-"`
}

func configPath(profile string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home directory: %w", err)
	}
	filename := configFile
	if profile != "" {
		filename = fmt.Sprintf("config-%s.json", profile)
	}
	return filepath.Join(home, configDir, filename), nil
}

func Load(profile string) (*Config, error) {
	path, err := configPath(profile)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Profile: profile}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.Profile = profile
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath(c.Profile)
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

func (c *Config) profileFlag() string {
	if c.Profile == "" {
		return ""
	}
	return " --profile " + c.Profile
}

func (c *Config) Validate() error {
	pf := c.profileFlag()
	if c.Server == "" {
		return fmt.Errorf("not logged in. Run: hawkeye%s login <server-url> -u <username> -p <password>", pf)
	}
	if c.Token == "" {
		return fmt.Errorf("not authenticated. Run: hawkeye%s login <server-url> -u <username> -p <password>", pf)
	}
	return nil
}

func (c *Config) ValidateProject() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.ProjectID == "" {
		return fmt.Errorf("project not set. Run: hawkeye%s set project <uuid>", c.profileFlag())
	}
	return nil
}

func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}
	dir := filepath.Join(home, configDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config directory: %w", err)
	}
	var profiles []string
	for _, e := range entries {
		name := e.Name()
		if name == configFile {
			profiles = append(profiles, "default")
			continue
		}
		if strings.HasPrefix(name, "config-") && strings.HasSuffix(name, ".json") {
			profiles = append(profiles, strings.TrimSuffix(strings.TrimPrefix(name, "config-"), ".json"))
		}
	}
	return profiles, nil
}

func ProfileName(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}
