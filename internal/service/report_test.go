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

func TestFormatSessionReport(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		got := FormatSessionReport(nil)
		if got.SessionID != "" {
			t.Errorf("SessionID = %q, want empty", got.SessionID)
		}
	})

	t.Run("full report", func(t *testing.T) {
		resp := &api.SessionReportResponse{
			SessionID: "s1",
			Summary:   "Root cause identified",
			TimeSaved: &api.TimeSavedSummary{
				TimeSavedMinutes:         25.0,
				StandardInvestigationMin: 30.0,
				HawkeyeInvestigationMin:  5.0,
			},
			Score: &api.AnalysisScore{
				Accuracy:     api.ScoreSection{Score: 0.95},
				Completeness: api.ScoreSection{Score: 0.88},
			},
		}

		got := FormatSessionReport(resp)
		if got.SessionID != "s1" {
			t.Errorf("SessionID = %q, want %q", got.SessionID, "s1")
		}
		if got.Summary != "Root cause identified" {
			t.Errorf("Summary = %q, want %q", got.Summary, "Root cause identified")
		}
		if got.TimeSaved == nil {
			t.Fatal("TimeSaved is nil, want non-nil")
		}
		if got.TimeSaved.TimeSavedMinutes != 25.0 {
			t.Errorf("TimeSavedMinutes = %f, want 25.0", got.TimeSaved.TimeSavedMinutes)
		}
		if !got.HasScores {
			t.Error("HasScores = false, want true")
		}
		if got.Accuracy != 0.95 {
			t.Errorf("Accuracy = %f, want 0.95", got.Accuracy)
		}
		if got.Completeness != 0.88 {
			t.Errorf("Completeness = %f, want 0.88", got.Completeness)
		}
	})

	t.Run("no scores", func(t *testing.T) {
		resp := &api.SessionReportResponse{
			SessionID: "s2",
			Summary:   "Summary only",
		}

		got := FormatSessionReport(resp)
		if got.HasScores {
			t.Error("HasScores = true, want false")
		}
		if got.TimeSaved != nil {
			t.Error("TimeSaved should be nil")
		}
	})
}
