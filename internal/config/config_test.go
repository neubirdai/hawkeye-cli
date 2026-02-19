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

	path := filepath.Join(tmpDir, configDir, configFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config file permissions = %o, want 0600", perm)
	}

	loaded, err := Load("")
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

	cfg, err := Load("")
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

func TestLoadSaveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &Config{
		Server:    "http://staging.example.com",
		Username:  "staging@test.com",
		Token:     "staging-token",
		OrgUUID:   "staging-org",
		ProjectID: "staging-proj",
		Profile:   "staging",
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path := filepath.Join(tmpDir, configDir, "config-staging.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("profile config file not created at %s: %v", path, err)
	}

	defaultPath := filepath.Join(tmpDir, configDir, configFile)
	if _, err := os.Stat(defaultPath); err == nil {
		t.Error("default config file should not exist")
	}

	loaded, err := Load("staging")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server != original.Server {
		t.Errorf("Server = %q, want %q", loaded.Server, original.Server)
	}
	if loaded.Profile != "staging" {
		t.Errorf("Profile = %q, want %q", loaded.Profile, "staging")
	}
}

func TestProfileIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	a := &Config{Server: "http://a.com", Token: "tok-a", Profile: "a"}
	b := &Config{Server: "http://b.com", Token: "tok-b", Profile: "b"}

	if err := a.Save(); err != nil {
		t.Fatalf("Save(a) error = %v", err)
	}
	if err := b.Save(); err != nil {
		t.Fatalf("Save(b) error = %v", err)
	}

	loadedA, err := Load("a")
	if err != nil {
		t.Fatalf("Load(a) error = %v", err)
	}
	loadedB, err := Load("b")
	if err != nil {
		t.Fatalf("Load(b) error = %v", err)
	}

	if loadedA.Server != "http://a.com" {
		t.Errorf("profile a Server = %q, want %q", loadedA.Server, "http://a.com")
	}
	if loadedB.Server != "http://b.com" {
		t.Errorf("profile b Server = %q, want %q", loadedB.Server, "http://b.com")
	}
}

func TestProfileName(t *testing.T) {
	tests := []struct {
		profile string
		want    string
	}{
		{"", "default"},
		{"staging", "staging"},
		{"prod", "prod"},
	}
	for _, tt := range tests {
		got := ProfileName(tt.profile)
		if got != tt.want {
			t.Errorf("ProfileName(%q) = %q, want %q", tt.profile, got, tt.want)
		}
	}
}

func TestValidateProfileHint(t *testing.T) {
	cfg := Config{Profile: "staging"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	want := "--profile staging"
	if got := err.Error(); !contains(got, want) {
		t.Errorf("Validate() error = %q, should contain %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
