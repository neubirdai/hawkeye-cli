package incidents

import (
	"fmt"
	"strings"
)

// PagerDutyProvider creates incidents via the PagerDuty Events API v2.
//
// Authentication: Creds.RoutingKey (integration routing key).
// Endpoint:       https://events.pagerduty.com/v2/enqueue
//
// Each incident is enqueued as an "alert" event. PagerDuty groups alerts into
// incidents automatically according to the service's alert grouping rules.
type PagerDutyProvider struct {
	Creds Creds
}

const pagerDutyDefaultBase = "https://events.pagerduty.com"

func (p *PagerDutyProvider) baseURL() string {
	if p.Creds.BaseURL != "" {
		return strings.TrimRight(p.Creds.BaseURL, "/")
	}
	return pagerDutyDefaultBase
}

// CreateIncidents implements IncidentProvider.
func (p *PagerDutyProvider) CreateIncidents(filename string, input IncidentInput) ([]CreatedIncident, error) {
	all, err := loadDataset(filename)
	if err != nil {
		return nil, err
	}

	var created []CreatedIncident
	for _, inc := range pick(all, input.Count) {
		// PagerDuty Events API v2 — the routing key is embedded in the body,
		// not in the Authorization header. Fall back to ApiKey if RoutingKey
		// is not explicitly set (they are the same credential).
		routingKey := p.Creds.RoutingKey
		if routingKey == "" {
			routingKey = p.Creds.ApiKey
		}
		body := map[string]any{
			"routing_key":  routingKey,
			"event_action": "trigger",
			// dedup_key lets PagerDuty de-duplicate alerts across retries.
			"dedup_key": inc.ID,
			"payload": map[string]any{
				"summary":        inc.Title,
				"severity":       inc.PagerDuty.Severity, // critical | error | warning | info
				"source":         inc.PagerDuty.Source,
				"component":      inc.PagerDuty.Component,
				"group":          inc.PagerDuty.Group,
				"custom_details": inc.PagerDuty.CustomDetails,
			},
		}

		resp, err := postJSON(p.baseURL()+"/v2/enqueue", nil, body)
		if err != nil {
			return created, fmt.Errorf("pagerduty: %s: %w", inc.ID, err)
		}

		// Successful response: {"status":"success","dedup_key":"...","message":"Event processed"}
		created = append(created, CreatedIncident{
			SourceID: inc.ID,
			RemoteID: stringField(resp, "dedup_key"),
			Title:    inc.Title,
		})
	}
	return created, nil
}
