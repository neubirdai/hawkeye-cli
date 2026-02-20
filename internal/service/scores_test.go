package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestExtractScores(t *testing.T) {
	tests := []struct {
		name      string
		resp      *api.GetSessionSummaryResponse
		wantHas   bool
		wantScore float64
	}{
		{
			name:    "nil response",
			resp:    nil,
			wantHas: false,
		},
		{
			name:    "nil summary",
			resp:    &api.GetSessionSummaryResponse{},
			wantHas: false,
		},
		{
			name: "nil analysis score",
			resp: &api.GetSessionSummaryResponse{
				SessionSummary: &api.SessionSummary{},
			},
			wantHas: false,
		},
		{
			name: "with scores",
			resp: &api.GetSessionSummaryResponse{
				SessionSummary: &api.SessionSummary{
					AnalysisScore: &api.AnalysisScore{
						ScoredBy: "auto",
						Accuracy: api.ScoreSection{
							Score:   85.5,
							Summary: "Good accuracy",
						},
						Completeness: api.ScoreSection{
							Score:   90.0,
							Summary: "Very complete",
						},
						Qualitative: api.QualSection{
							Strengths:    []string{"thorough", "fast"},
							Improvements: []string{"more sources"},
						},
					},
				},
			},
			wantHas:   true,
			wantScore: 85.5,
		},
		{
			name: "with time saved",
			resp: &api.GetSessionSummaryResponse{
				SessionSummary: &api.SessionSummary{
					AnalysisScore: &api.AnalysisScore{
						Accuracy: api.ScoreSection{Score: 75.0},
					},
					TimeSaved: &api.TimeSavedSummary{
						TimeSavedMinutes:         30.0,
						StandardInvestigationMin: 45.0,
						HawkeyeInvestigationMin:  15.0,
					},
				},
			},
			wantHas:   true,
			wantScore: 75.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractScores(tt.resp)
			if got.HasScores != tt.wantHas {
				t.Errorf("HasScores = %v, want %v", got.HasScores, tt.wantHas)
			}
			if tt.wantHas && got.Accuracy.Score != tt.wantScore {
				t.Errorf("Accuracy.Score = %v, want %v", got.Accuracy.Score, tt.wantScore)
			}
		})
	}

	// Test specific fields in detail
	t.Run("detailed field check", func(t *testing.T) {
		resp := &api.GetSessionSummaryResponse{
			SessionSummary: &api.SessionSummary{
				AnalysisScore: &api.AnalysisScore{
					ScoredBy: "human",
					Accuracy: api.ScoreSection{
						Score:   88.0,
						Summary: "Accurate",
					},
					Completeness: api.ScoreSection{
						Score:   92.0,
						Summary: "Complete",
					},
					Qualitative: api.QualSection{
						Strengths:    []string{"a", "b"},
						Improvements: []string{"c"},
					},
				},
				TimeSaved: &api.TimeSavedSummary{
					TimeSavedMinutes:         25.0,
					StandardInvestigationMin: 40.0,
					HawkeyeInvestigationMin:  15.0,
				},
			},
		}

		got := ExtractScores(resp)
		if got.ScoredBy != "human" {
			t.Errorf("ScoredBy = %q, want %q", got.ScoredBy, "human")
		}
		if got.Completeness.Score != 92.0 {
			t.Errorf("Completeness.Score = %v, want %v", got.Completeness.Score, 92.0)
		}
		if len(got.Qualitative.Strengths) != 2 {
			t.Errorf("Strengths len = %d, want 2", len(got.Qualitative.Strengths))
		}
		if got.TimeSaved == nil {
			t.Fatal("TimeSaved is nil, want non-nil")
		}
		if got.TimeSaved.TimeSavedMinutes != 25.0 {
			t.Errorf("TimeSavedMinutes = %v, want %v", got.TimeSaved.TimeSavedMinutes, 25.0)
		}
	})
}
