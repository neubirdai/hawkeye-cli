package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFormatReport(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		got := FormatReport(nil)
		if got.TotalIncidents != 0 {
			t.Errorf("TotalIncidents = %d, want 0", got.TotalIncidents)
		}
	})

	t.Run("full report", func(t *testing.T) {
		resp := &api.IncidentReportResponse{
			AvgTimeSavedMinutes: 15.5,
			AvgMTTR:             30.2,
			NoiseReduction:      0.45,
			TotalIncidents:      100,
			TotalInvestigations: 150,
			TotalTimeSavedHours: 38.75,
			StartTime:           "2025-01-01",
			EndTime:             "2025-06-30",
			IncidentTypeReports: []api.IncidentTypeReport{
				{Type: "alert", Count: 50, AvgTimeSavedMinutes: 10.0, NoiseReduction: 0.3},
				{Type: "incident", Count: 50, AvgTimeSavedMinutes: 20.0, NoiseReduction: 0.6},
			},
		}

		got := FormatReport(resp)
		if got.AvgTimeSavedMinutes != "15.5 min" {
			t.Errorf("AvgTimeSavedMinutes = %q, want %q", got.AvgTimeSavedMinutes, "15.5 min")
		}
		if got.AvgMTTR != "30.2 min" {
			t.Errorf("AvgMTTR = %q, want %q", got.AvgMTTR, "30.2 min")
		}
		if got.NoiseReduction != "45.0%" {
			t.Errorf("NoiseReduction = %q, want %q", got.NoiseReduction, "45.0%")
		}
		if got.TotalIncidents != 100 {
			t.Errorf("TotalIncidents = %d, want 100", got.TotalIncidents)
		}
		if got.TotalTimeSavedHours != "38.8 hrs" {
			t.Errorf("TotalTimeSavedHours = %q, want %q", got.TotalTimeSavedHours, "38.8 hrs")
		}
		if got.Period != "2025-01-01 to 2025-06-30" {
			t.Errorf("Period = %q, want %q", got.Period, "2025-01-01 to 2025-06-30")
		}
		if len(got.IncidentTypes) != 2 {
			t.Fatalf("IncidentTypes len = %d, want 2", len(got.IncidentTypes))
		}
		if got.IncidentTypes[0].Type != "alert" {
			t.Errorf("IncidentTypes[0].Type = %q, want %q", got.IncidentTypes[0].Type, "alert")
		}
		if got.IncidentTypes[1].AvgTimeSaved != "20.0 min" {
			t.Errorf("IncidentTypes[1].AvgTimeSaved = %q, want %q", got.IncidentTypes[1].AvgTimeSaved, "20.0 min")
		}
	})

	t.Run("no period", func(t *testing.T) {
		resp := &api.IncidentReportResponse{
			TotalIncidents: 5,
		}
		got := FormatReport(resp)
		if got.Period != "" {
			t.Errorf("Period = %q, want empty", got.Period)
		}
	})
}
