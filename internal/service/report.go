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
	Type       string
	Priorities []PriorityDisplay
}

// PriorityDisplay holds display-ready priority breakdown data.
type PriorityDisplay struct {
	Priority       string
	TotalIncidents int
	Investigated   int
	PercentGrouped string
	AvgTimeSaved   string
}

// FormatReport transforms raw analytics into a display-ready struct.
func FormatReport(resp *api.IncidentReportResponse) ReportDisplay {
	if resp == nil {
		return ReportDisplay{}
	}

	display := ReportDisplay{
		AvgTimeSavedMinutes: fmt.Sprintf("%.1f min", resp.AvgTimeSavedMinutes),
		AvgMTTR:             fmt.Sprintf("%.1f min", resp.AvgMTTR),
		NoiseReduction:      fmt.Sprintf("%.1f%%", resp.NoiseReduction),
		TotalIncidents:      resp.TotalIncidents,
		TotalInvestigations: resp.TotalInvestigations,
		TotalTimeSavedHours: fmt.Sprintf("%.1f hrs", resp.TotalTimeSavedHours),
	}

	if resp.StartTime != "" && resp.EndTime != "" {
		display.Period = fmt.Sprintf("%s to %s", resp.StartTime, resp.EndTime)
	}

	for _, itr := range resp.IncidentTypeReports {
		itd := IncidentTypeDisplay{Type: itr.Type}
		for _, pr := range itr.PriorityReports {
			itd.Priorities = append(itd.Priorities, PriorityDisplay{
				Priority:       pr.Priority,
				TotalIncidents: pr.TotalIncidents,
				Investigated:   pr.InvestigatedIncidents,
				PercentGrouped: fmt.Sprintf("%.1f%%", pr.PercentGrouped),
				AvgTimeSaved:   fmt.Sprintf("%.1f min", pr.AvgTimeSavedMinutes),
			})
		}
		display.IncidentTypes = append(display.IncidentTypes, itd)
	}

	return display
}
