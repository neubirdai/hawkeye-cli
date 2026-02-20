package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFormatConnection(t *testing.T) {
	tests := []struct {
		name     string
		input    api.ConnectionSpec
		wantName string
		wantType string
	}{
		{
			name:     "named connection",
			input:    api.ConnectionSpec{UUID: "c1", Name: "Datadog Prod", Type: "datadog", SyncState: "SYNCED"},
			wantName: "Datadog Prod",
			wantType: "datadog",
		},
		{
			name:     "unnamed connection",
			input:    api.ConnectionSpec{UUID: "c2", Type: "prometheus"},
			wantName: "(unnamed)",
			wantType: "prometheus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConnection(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.UUID != tt.input.UUID {
				t.Errorf("UUID = %q, want %q", got.UUID, tt.input.UUID)
			}
		})
	}
}

func TestFormatResources(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		got := FormatResources(nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("with resources", func(t *testing.T) {
		specs := []api.ResourceSpec{
			{
				ID:             api.ResourceID{Name: "cpu-metric", UUID: "r1"},
				ConnectionUUID: "c1",
				TelemetryType:  "metric",
			},
			{
				ID:             api.ResourceID{Name: "", UUID: "r2"},
				ConnectionUUID: "c1",
				TelemetryType:  "log",
			},
		}

		got := FormatResources(specs)
		if len(got) != 2 {
			t.Fatalf("got %d resources, want 2", len(got))
		}
		if got[0].Name != "cpu-metric" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "cpu-metric")
		}
		if got[1].Name != "r2" {
			t.Errorf("got[1].Name = %q, want %q (should fall back to UUID)", got[1].Name, "r2")
		}
		if got[0].TelemetryType != "metric" {
			t.Errorf("got[0].TelemetryType = %q, want %q", got[0].TelemetryType, "metric")
		}
	})
}
