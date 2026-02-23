package incidents

import (
	"fmt"
	"strings"
)

// IncidentIOProvider creates incidents via the incident.io REST API.
//
// Authentication: Creds.ApiKey as a bearer token.
// Endpoint:       https://api.incident.io/v2/incidents
type IncidentIOProvider struct {
	Creds Creds
}

const incidentIODefaultBase = "https://api.incident.io"

func (i *IncidentIOProvider) baseURL() string {
	if i.Creds.BaseURL != "" {
		return strings.TrimRight(i.Creds.BaseURL, "/")
	}
	return incidentIODefaultBase
}

func (i *IncidentIOProvider) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + i.Creds.ApiKey,
	}
}

// CreateIncidents implements IncidentProvider.
func (i *IncidentIOProvider) CreateIncidents(filename string, input IncidentInput) ([]CreatedIncident, error) {
	all, err := loadDataset(filename)
	if err != nil {
		return nil, err
	}

	var created []CreatedIncident
	for _, inc := range pick(all, input.Count) {
		// incident.io POST /v2/incidents
		// https://api-docs.incident.io/reference/incidents-v2-create
		//
		// severity_id must be a valid ID from GET /v2/severities; we use the
		// human-readable label here and rely on the platform to match it.
		// mode: "real" | "test" | "tutorial"
		body := map[string]any{
			"name":            inc.Title,
			"summary":         inc.Description,
			"mode":            inc.IncidentIO.Mode,
			// idempotency_key prevents duplicate incidents if the request is retried.
			"idempotency_key": inc.ID,
			"severity": map[string]any{
				"name": inc.IncidentIO.Severity,
			},
		}

		// Attach custom fields if provided.
		if len(inc.IncidentIO.CustomFields) > 0 {
			body["custom_field_entries"] = inc.IncidentIO.CustomFields
		}

		resp, err := postJSON(i.baseURL()+"/v2/incidents", i.headers(), body)
		if err != nil {
			return created, fmt.Errorf("incidentio: %s: %w", inc.ID, err)
		}

		// Successful response: { "incident": { "id": "...", "reference": "INC-123", "permalink": "..." } }
		created = append(created, CreatedIncident{
			SourceID: inc.ID,
			RemoteID: stringField(resp, "incident", "id"),
			Title:    inc.Title,
			URL:      stringField(resp, "incident", "permalink"),
		})
	}
	return created, nil
}
