package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFormatDiscoveredResources(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		got := FormatDiscoveredResources(nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("with resources", func(t *testing.T) {
		specs := []api.ResourceSpec{
			{ID: api.ResourceID{Name: "cpu-metric", UUID: "r1"}, ConnectionUUID: "c1", TelemetryType: "metric"},
			{ID: api.ResourceID{Name: "", UUID: "r2"}, ConnectionUUID: "c2", TelemetryType: "log"},
		}
		got := FormatDiscoveredResources(specs)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
		if got[0].Name != "cpu-metric" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "cpu-metric")
		}
		if got[1].Name != "r2" {
			t.Errorf("got[1].Name = %q, want %q (fallback to UUID)", got[1].Name, "r2")
		}
	})
}

func TestGetResourceTypes(t *testing.T) {
	t.Run("specific connection and telemetry", func(t *testing.T) {
		got := GetResourceTypes("aws", "metric")
		if len(got) == 0 {
			t.Error("expected non-empty result for aws/metric")
		}
	})

	t.Run("connection only", func(t *testing.T) {
		got := GetResourceTypes("datadog", "")
		if len(got) == 0 {
			t.Error("expected non-empty result for datadog")
		}
	})

	t.Run("all types", func(t *testing.T) {
		got := GetResourceTypes("", "")
		if len(got) == 0 {
			t.Error("expected non-empty result for all types")
		}
	})

	t.Run("unknown connection", func(t *testing.T) {
		got := GetResourceTypes("unknown", "metric")
		if got != nil {
			t.Errorf("got %v, want nil for unknown", got)
		}
	})
}
