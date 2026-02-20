package service

import (
	"hawkeye-cli/internal/api"
)

// ScoreDisplay holds display-ready score information.
type ScoreDisplay struct {
	HasScores    bool
	ScoredBy     string
	Accuracy     ScoreSectionDisplay
	Completeness ScoreSectionDisplay
	Qualitative  QualSectionDisplay
	TimeSaved    *TimeSavedDisplay
}

// ScoreSectionDisplay holds a numeric score section.
type ScoreSectionDisplay struct {
	Score   float64
	Summary string
}

// QualSectionDisplay holds a qualitative score section.
type QualSectionDisplay struct {
	Strengths    []string
	Improvements []string
}

// TimeSavedDisplay holds time-saved summary data.
type TimeSavedDisplay struct {
	TimeSavedMinutes         float64
	StandardInvestigationMin float64
	HawkeyeInvestigationMin  float64
}

// ExtractScores pulls accuracy/completeness/qualitative from the summary response.
func ExtractScores(resp *api.GetSessionSummaryResponse) ScoreDisplay {
	if resp == nil || resp.SessionSummary == nil || resp.SessionSummary.AnalysisScore == nil {
		return ScoreDisplay{HasScores: false}
	}

	score := resp.SessionSummary.AnalysisScore
	display := ScoreDisplay{
		HasScores: true,
		ScoredBy:  score.ScoredBy,
		Accuracy: ScoreSectionDisplay{
			Score:   score.Accuracy.Score,
			Summary: score.Accuracy.Summary,
		},
		Completeness: ScoreSectionDisplay{
			Score:   score.Completeness.Score,
			Summary: score.Completeness.Summary,
		},
		Qualitative: QualSectionDisplay{
			Strengths:    score.Qualitative.Strengths,
			Improvements: score.Qualitative.Improvements,
		},
	}

	if resp.SessionSummary.TimeSaved != nil {
		display.TimeSaved = &TimeSavedDisplay{
			TimeSavedMinutes:         resp.SessionSummary.TimeSaved.TimeSavedMinutes,
			StandardInvestigationMin: resp.SessionSummary.TimeSaved.StandardInvestigationMin,
			HawkeyeInvestigationMin:  resp.SessionSummary.TimeSaved.HawkeyeInvestigationMin,
		}
	}

	return display
}
