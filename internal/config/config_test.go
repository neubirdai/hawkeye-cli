package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     Config{Server: "http://localhost:3001", Token: "abc123"},
			wantErr: false,
		},
		{
			name:    "missing server",
			cfg:     Config{Token: "abc123"},
			wantErr: true,
		},
		{
			name:    "missing token",
			cfg:     Config{Server: "http://localhost:3001"},
			wantErr: true,
		},
		{
			name:    "both missing",
			cfg:     Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProject(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "fully valid",
			cfg:     Config{Server: "http://localhost", Token: "tok", ProjectID: "proj-123"},
			wantErr: false,
		},
		{
			name:    "missing project",
			cfg:     Config{Server: "http://localhost", Token: "tok"},
			wantErr: true,
		},
		{
			name:    "missing server (fails Validate first)",
			cfg:     Config{Token: "tok", ProjectID: "proj"},
			wantErr: true,
		},
		{
			name:    "missing token (fails Validate first)",
			cfg:     Config{Server: "http://localhost", ProjectID: "proj"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateProject()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &Config{
		Server:    "http://example.com",
		Username:  "user@test.com",
		Token:     "jwt-token-here",
		OrgUUID:   "org-uuid-123",
		ProjectID: "proj-uuid-456",
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists with correct permissions
	path := filepath.Join(tmpDir, configDir, configFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config file permissions = %o, want 0600", perm)
	}

	// Load and compare
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Server != original.Server {
		t.Errorf("Server = %q, want %q", loaded.Server, original.Server)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username = %q, want %q", loaded.Username, original.Username)
	}
	if loaded.Token != original.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, original.Token)
	}
	if loaded.OrgUUID != original.OrgUUID {
		t.Errorf("OrgUUID = %q, want %q", loaded.OrgUUID, original.OrgUUID)
	}
	if loaded.ProjectID != original.ProjectID {
		t.Errorf("ProjectID = %q, want %q", loaded.ProjectID, original.ProjectID)
	}
}

func TestLoadMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing config returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Server != "" || cfg.Token != "" || cfg.ProjectID != "" {
		t.Errorf("Load() on missing config returned non-empty fields: %+v", cfg)
	}
}
