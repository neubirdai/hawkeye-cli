package service

import (
	"hawkeye-cli/internal/api"
)

// ConnectionDisplay holds display-ready connection info.
type ConnectionDisplay struct {
	UUID          string
	Name          string
	Type          string
	SyncState     string
	TrainingState string
}

// ResourceDisplay holds display-ready resource info.
type ResourceDisplay struct {
	Name           string
	ConnectionUUID string
	TelemetryType  string
}

// FormatConnection maps a raw ConnectionSpec to a display-ready struct.
func FormatConnection(c api.ConnectionSpec) ConnectionDisplay {
	name := c.Name
	if name == "" {
		name = "(unnamed)"
	}

	return ConnectionDisplay{
		UUID:          c.UUID,
		Name:          name,
		Type:          c.Type,
		SyncState:     c.SyncState,
		TrainingState: c.TrainingState,
	}
}

// FormatResources maps raw ResourceSpecs to display-ready structs.
func FormatResources(specs []api.ResourceSpec) []ResourceDisplay {
	var result []ResourceDisplay
	for _, r := range specs {
		name := r.ID.Name
		if name == "" {
			name = r.ID.UUID
		}
		result = append(result, ResourceDisplay{
			Name:           name,
			ConnectionUUID: r.ConnectionUUID,
			TelemetryType:  r.TelemetryType,
		})
	}
	return result
}
