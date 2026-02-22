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
			NoiseReduction:      45.0,
			TotalIncidents:      100,
			TotalInvestigations: 150,
			TotalTimeSavedHours: 38.75,
			StartTime:           "2025-01-01",
			EndTime:             "2025-06-30",
			IncidentTypeReports: []api.IncidentTypeReport{
				{
					Type: "Datadog Monitor",
					PriorityReports: []api.PriorityReport{
						{Priority: "0", TotalIncidents: 50, PercentGrouped: 80.6, AvgTimeSavedMinutes: 10.0},
					},
				},
				{
					Type: "User Initiated Session",
					PriorityReports: []api.PriorityReport{
						{Priority: "Unspecified", InvestigatedIncidents: 24, AvgTimeSavedMinutes: 89.6},
					},
				},
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
		if got.IncidentTypes[0].Type != "Datadog Monitor" {
			t.Errorf("IncidentTypes[0].Type = %q, want %q", got.IncidentTypes[0].Type, "Datadog Monitor")
		}
		if len(got.IncidentTypes[0].Priorities) != 1 {
			t.Fatalf("IncidentTypes[0].Priorities len = %d, want 1", len(got.IncidentTypes[0].Priorities))
		}
		if got.IncidentTypes[0].Priorities[0].PercentGrouped != "80.6%" {
			t.Errorf("PercentGrouped = %q, want %q", got.IncidentTypes[0].Priorities[0].PercentGrouped, "80.6%")
		}
		if got.IncidentTypes[1].Priorities[0].AvgTimeSaved != "89.6 min" {
			t.Errorf("AvgTimeSaved = %q, want %q", got.IncidentTypes[1].Priorities[0].AvgTimeSaved, "89.6 min")
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
