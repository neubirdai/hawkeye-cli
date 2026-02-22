package service

import (
	"hawkeye-cli/internal/api"
)

// InstructionDisplay holds display-ready instruction info.
type InstructionDisplay struct {
	UUID       string
	Name       string
	Type       string
	Content    string
	Enabled    bool
	CreateTime string
}

// FormatInstruction maps a raw InstructionSpec to a display-ready struct.
func FormatInstruction(i api.InstructionSpec) InstructionDisplay {
	name := i.Name
	if name == "" {
		name = "(unnamed)"
	}
	return InstructionDisplay{
		UUID:       i.UUID,
		Name:       name,
		Type:       i.Type,
		Content:    i.Content,
		Enabled:    i.Enabled,
		CreateTime: i.CreateTime,
	}
}

// FormatInstructions maps a list of raw InstructionSpecs to display-ready structs.
func FormatInstructions(specs []api.InstructionSpec) []InstructionDisplay {
	var result []InstructionDisplay
	for _, s := range specs {
		result = append(result, FormatInstruction(s))
	}
	return result
}

// InstructionTypes returns the valid instruction types.
func InstructionTypes() []string {
	return []string{"filter", "system", "grouping", "rca"}
}

// ValidInstructionType checks if a type is valid.
func ValidInstructionType(t string) bool {
	for _, valid := range InstructionTypes() {
		if t == valid {
			return true
		}
	}
	return false
}
