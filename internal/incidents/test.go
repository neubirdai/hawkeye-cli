package incidents

import (
	"fmt"
	"strings"
)

// RunTest creates test incidents of the given provider type using the provided
// credentials and dataset file.
//
// providerType must be one of "pagerduty", "firehydrant", or "incidentio".
// input.Count controls how many incidents are created (0 = all).
func RunTest(providerType string, creds Creds, filename string, input IncidentInput) ([]CreatedIncident, error) {
	var p IncidentProvider
	switch strings.ToLower(providerType) {
	case "pagerduty":
		p = &PagerDutyProvider{Creds: creds}
	case "firehydrant":
		p = &FireHydrantProvider{Creds: creds}
	case "incidentio", "incident.io":
		p = &IncidentIOProvider{Creds: creds}
	default:
		return nil, fmt.Errorf("unknown provider type %q: choose pagerduty, firehydrant, or incidentio", providerType)
	}
	return p.CreateIncidents(filename, input)
}
