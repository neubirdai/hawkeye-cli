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

// ConnectionDetailDisplay holds display-ready connection detail info.
type ConnectionDetailDisplay struct {
	UUID          string
	Name          string
	Type          string
	SyncState     string
	TrainingState string
	CreateTime    string
	UpdateTime    string
}

// FormatConnectionDetail maps a raw ConnectionDetail to a display-ready struct.
func FormatConnectionDetail(c *api.ConnectionDetail) ConnectionDetailDisplay {
	if c == nil {
		return ConnectionDetailDisplay{}
	}
	name := c.Name
	if name == "" {
		name = "(unnamed)"
	}
	return ConnectionDetailDisplay{
		UUID:          c.UUID,
		Name:          name,
		Type:          c.Type,
		SyncState:     c.SyncState,
		TrainingState: c.TrainingState,
		CreateTime:    c.CreateTime,
		UpdateTime:    c.UpdateTime,
	}
}

// ConnectionType describes a supported connection type.
type ConnectionType struct {
	Type        string
	Description string
}

// GetConnectionTypes returns the list of supported connection types.
func GetConnectionTypes() []ConnectionType {
	return []ConnectionType{
		{"aws", "Amazon Web Services (CloudWatch, X-Ray)"},
		{"datadog", "Datadog monitoring platform"},
		{"prometheus", "Prometheus metrics"},
		{"grafana", "Grafana dashboards and datasources"},
		{"pagerduty", "PagerDuty incident management"},
		{"jira", "Jira issue tracking"},
		{"slack", "Slack notifications"},
		{"elasticsearch", "Elasticsearch / OpenSearch logs"},
		{"gcp", "Google Cloud Platform (Cloud Monitoring)"},
		{"azure", "Microsoft Azure Monitor"},
		{"splunk", "Splunk observability"},
		{"newrelic", "New Relic monitoring"},
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
