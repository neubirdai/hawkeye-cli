package incidents

import (
	"fmt"
	"strings"
)

// FireHydrantProvider creates incidents via the FireHydrant REST API.
//
// Authentication: Creds.ApiKey as a bearer token.
// Endpoint:       https://api.firehydrant.io/v1/incidents
type FireHydrantProvider struct {
	Creds Creds
}

const fireHydrantDefaultBase = "https://api.firehydrant.io"

func (f *FireHydrantProvider) baseURL() string {
	if f.Creds.BaseURL != "" {
		return strings.TrimRight(f.Creds.BaseURL, "/")
	}
	return fireHydrantDefaultBase
}

func (f *FireHydrantProvider) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + f.Creds.ApiKey,
	}
}

// CreateIncidents implements IncidentProvider.
func (f *FireHydrantProvider) CreateIncidents(filename string, input IncidentInput) ([]CreatedIncident, error) {
	all, err := loadDataset(filename)
	if err != nil {
		return nil, err
	}

	var created []CreatedIncident
	for _, inc := range pick(all, input.Count) {
		// FireHydrant POST /v1/incidents
		// https://developers.firehydrant.io/docs/api/b3A6NDYxMTk5Mjk-create-an-incident
		body := map[string]any{
			"name":        inc.Title,
			"description": inc.Description,
			// severity: SEV1 | SEV2 | SEV3 | SEV4
			"severity": inc.FireHydrant.Severity,
			// tag_list is a comma-separated string of tags
			"tag_list": strings.Join(inc.FireHydrant.Labels, ","),
		}

		// Attach affected services as impacted infrastructure if provided.
		if len(inc.FireHydrant.AffectedServices) > 0 {
			services := make([]map[string]any, 0, len(inc.FireHydrant.AffectedServices))
			for _, svc := range inc.FireHydrant.AffectedServices {
				services = append(services, map[string]any{"name": svc})
			}
			body["impacted_services"] = services
		}

		resp, err := postJSON(f.baseURL()+"/v1/incidents", f.headers(), body)
		if err != nil {
			return created, fmt.Errorf("firehydrant: %s: %w", inc.ID, err)
		}

		// Successful response contains the created incident object.
		created = append(created, CreatedIncident{
			SourceID: inc.ID,
			RemoteID: stringField(resp, "id"),
			Title:    inc.Title,
			URL:      stringField(resp, "incident_url"),
		})
	}
	return created, nil
}
