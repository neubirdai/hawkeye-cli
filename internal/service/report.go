package service

import (
	"fmt"

	"hawkeye-cli/internal/api"
)

// ReportDisplay holds display-ready incident report data.
type ReportDisplay struct {
	AvgTimeSavedMinutes string
	AvgMTTR             string
	NoiseReduction      string
	TotalIncidents      int
	TotalInvestigations int
	TotalTimeSavedHours string
	Period              string
	IncidentTypes       []IncidentTypeDisplay
}

// IncidentTypeDisplay holds a single incident type's report data.
type IncidentTypeDisplay struct {
	Type           string
	Count          int
	AvgTimeSaved   string
	NoiseReduction string
}

// FormatReport transforms raw analytics into a display-ready struct.
func FormatReport(resp *api.IncidentReportResponse) ReportDisplay {
	if resp == nil {
		return ReportDisplay{}
	}

	display := ReportDisplay{
		AvgTimeSavedMinutes: fmt.Sprintf("%.1f min", resp.AvgTimeSavedMinutes),
		AvgMTTR:             fmt.Sprintf("%.1f min", resp.AvgMTTR),
		NoiseReduction:      fmt.Sprintf("%.1f%%", resp.NoiseReduction*100),
		TotalIncidents:      resp.TotalIncidents,
		TotalInvestigations: resp.TotalInvestigations,
		TotalTimeSavedHours: fmt.Sprintf("%.1f hrs", resp.TotalTimeSavedHours),
	}

	if resp.StartTime != "" && resp.EndTime != "" {
		display.Period = fmt.Sprintf("%s to %s", resp.StartTime, resp.EndTime)
	}

	for _, itr := range resp.IncidentTypeReports {
		display.IncidentTypes = append(display.IncidentTypes, IncidentTypeDisplay{
			Type:           itr.Type,
			Count:          itr.Count,
			AvgTimeSaved:   fmt.Sprintf("%.1f min", itr.AvgTimeSavedMinutes),
			NoiseReduction: fmt.Sprintf("%.1f%%", itr.NoiseReduction*100),
		})
	}

	return display
}

// SessionReportDisplay holds display-ready per-session report data.
type SessionReportDisplay struct {
	SessionID    string
	Summary      string
	TimeSaved    *TimeSavedDisplay
	HasScores    bool
	Accuracy     float64
	Completeness float64
}

// FormatSessionReport transforms raw session report into a display-ready struct.
func FormatSessionReport(resp *api.SessionReportResponse) SessionReportDisplay {
	if resp == nil {
		return SessionReportDisplay{}
	}

	display := SessionReportDisplay{
		SessionID: resp.SessionID,
		Summary:   resp.Summary,
	}

	if resp.TimeSaved != nil {
		display.TimeSaved = &TimeSavedDisplay{
			TimeSavedMinutes:         resp.TimeSaved.TimeSavedMinutes,
			StandardInvestigationMin: resp.TimeSaved.StandardInvestigationMin,
			HawkeyeInvestigationMin:  resp.TimeSaved.HawkeyeInvestigationMin,
		}
	}

	if resp.Score != nil {
		display.HasScores = true
		display.Accuracy = resp.Score.Accuracy.Score
		display.Completeness = resp.Score.Completeness.Score
	}

	return display
}
