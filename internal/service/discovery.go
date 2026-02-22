package service

import (
	"hawkeye-cli/internal/api"
)

// DiscoveredResource holds display-ready discovered resource info.
type DiscoveredResource struct {
	Name           string
	ConnectionUUID string
	TelemetryType  string
}

// FormatDiscoveredResources maps raw ResourceSpecs to display-ready structs.
func FormatDiscoveredResources(resources []api.ResourceSpec) []DiscoveredResource {
	var result []DiscoveredResource
	for _, r := range resources {
		name := r.ID.Name
		if name == "" {
			name = r.ID.UUID
		}
		result = append(result, DiscoveredResource{
			Name:           name,
			ConnectionUUID: r.ConnectionUUID,
			TelemetryType:  r.TelemetryType,
		})
	}
	return result
}

// ResourceType describes a telemetry resource type.
type ResourceType struct {
	Type        string
	Description string
}

// GetResourceTypes returns the resource types for a given connection type and telemetry type.
func GetResourceTypes(connectionType, telemetryType string) []ResourceType {
	allTypes := map[string]map[string][]ResourceType{
		"aws": {
			"metric": {
				{Type: "cloudwatch_metric", Description: "AWS CloudWatch Metric"},
				{Type: "cloudwatch_alarm", Description: "AWS CloudWatch Alarm"},
			},
			"log": {
				{Type: "cloudwatch_log_group", Description: "AWS CloudWatch Log Group"},
			},
			"trace": {
				{Type: "xray_trace", Description: "AWS X-Ray Trace"},
			},
		},
		"datadog": {
			"metric": {
				{Type: "datadog_metric", Description: "Datadog Metric"},
			},
			"log": {
				{Type: "datadog_log", Description: "Datadog Log"},
			},
			"trace": {
				{Type: "datadog_apm_trace", Description: "Datadog APM Trace"},
			},
		},
		"prometheus": {
			"metric": {
				{Type: "prometheus_metric", Description: "Prometheus Metric"},
			},
		},
	}

	if connectionType != "" && telemetryType != "" {
		if connTypes, ok := allTypes[connectionType]; ok {
			if telTypes, ok := connTypes[telemetryType]; ok {
				return telTypes
			}
		}
		return nil
	}

	if connectionType != "" {
		if connTypes, ok := allTypes[connectionType]; ok {
			var result []ResourceType
			for _, types := range connTypes {
				result = append(result, types...)
			}
			return result
		}
		return nil
	}

	// Return all
	var result []ResourceType
	for _, connTypes := range allTypes {
		for _, types := range connTypes {
			result = append(result, types...)
		}
	}
	return result
}
