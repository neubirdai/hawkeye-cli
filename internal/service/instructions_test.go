package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFormatInstruction(t *testing.T) {
	tests := []struct {
		name     string
		input    api.InstructionSpec
		wantName string
		wantType string
	}{
		{
			name:     "with name",
			input:    api.InstructionSpec{UUID: "i1", Name: "Filter Noise", Type: "filter", Content: "ignore dev", Enabled: true},
			wantName: "Filter Noise",
			wantType: "filter",
		},
		{
			name:     "unnamed",
			input:    api.InstructionSpec{UUID: "i2", Type: "system"},
			wantName: "(unnamed)",
			wantType: "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatInstruction(tt.input)
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

func TestFormatInstructions(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		got := FormatInstructions(nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("with items", func(t *testing.T) {
		specs := []api.InstructionSpec{
			{UUID: "i1", Name: "A", Type: "filter"},
			{UUID: "i2", Name: "B", Type: "system"},
		}
		got := FormatInstructions(specs)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
	})
}

func TestValidInstructionType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"filter", true},
		{"system", true},
		{"grouping", true},
		{"rca", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ValidInstructionType(tt.input)
			if got != tt.want {
				t.Errorf("ValidInstructionType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
