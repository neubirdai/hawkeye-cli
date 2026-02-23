// Package incidents defines the IncidentProvider interface and the shared
// types used by all platform implementations (PagerDuty, FireHydrant, incident.io).
package incidents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultDataset is the built-in dataset used when no --file is specified.
// It is populated by the main package at startup via go:embed.
var DefaultDataset []byte

// ─── Credentials ─────────────────────────────────────────────────────────────

// Creds generalises authentication across incident management platforms.
//
//   - PagerDuty Events API v2: set RoutingKey (the integration routing key).
//   - PagerDuty REST API:      set ApiKey.
//   - FireHydrant:             set ApiKey.
//   - incident.io:             set ApiKey.
//
// BaseURL is optional; it overrides the platform default (useful for staging
// environments or local test doubles).
type Creds struct {
	// ApiKey is the primary bearer credential used by FireHydrant and incident.io,
	// and by the PagerDuty REST API.
	ApiKey string

	// RoutingKey is the PagerDuty Events API v2 integration routing key.
	// When set it takes precedence over ApiKey for PagerDuty event ingestion.
	RoutingKey string

	// BaseURL overrides the default API base URL for the platform.
	BaseURL string
}

// ─── Input ───────────────────────────────────────────────────────────────────

// IncidentInput controls which incidents from the dataset are created.
type IncidentInput struct {
	// Count is the number of incidents to create.
	// 0 means create all incidents in the file.
	Count int
}

// ─── Result ──────────────────────────────────────────────────────────────────

// CreatedIncident records the outcome of a single incident creation call.
type CreatedIncident struct {
	// SourceID is the identifier from the YAML dataset (e.g. "INC-001").
	SourceID string

	// RemoteID is the identifier assigned by the remote platform.
	RemoteID string

	// Title is the incident title as sent to the platform.
	Title string

	// URL is a deep-link to the incident in the platform UI, when available.
	URL string
}

// ─── Interface ───────────────────────────────────────────────────────────────

// IncidentProvider can create incidents in a remote incident management platform.
// Each platform (PagerDuty, FireHydrant, incident.io) provides its own implementation.
type IncidentProvider interface {
	// CreateIncidents reads the YAML dataset at filename, selects the first
	// input.Count incidents (all if Count == 0), and creates them in the
	// remote platform using the receiver's credentials.
	//
	// It returns one CreatedIncident per successfully created entry.
	// On the first API error the function returns the incidents created so far
	// together with a wrapped error.
	CreateIncidents(filename string, input IncidentInput) ([]CreatedIncident, error)
}

// ─── YAML dataset model ───────────────────────────────────────────────────────

type dataset struct {
	Incidents []YAMLIncident `yaml:"incidents"`
}

// YAMLIncident mirrors one entry in the test-data YAML file.
type YAMLIncident struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Severity    string   `yaml:"severity"`
	Description string   `yaml:"description"`
	Service     string   `yaml:"service"`
	Environment string   `yaml:"environment"`
	Team        string   `yaml:"team"`
	Tags        []string `yaml:"tags"`

	PagerDuty struct {
		Severity      string         `yaml:"severity"`
		Source        string         `yaml:"source"`
		Component     string         `yaml:"component"`
		Group         string         `yaml:"group"`
		CustomDetails map[string]any `yaml:"custom_details"`
	} `yaml:"pagerduty"`

	FireHydrant struct {
		Severity         string   `yaml:"severity"`
		Labels           []string `yaml:"labels"`
		AffectedServices []string `yaml:"affected_services"`
	} `yaml:"firehydrant"`

	IncidentIO struct {
		Severity     string         `yaml:"severity"`
		Mode         string         `yaml:"mode"`
		CustomFields map[string]any `yaml:"custom_fields"`
	} `yaml:"incidentio"`
}

// loadDataset parses the YAML file and returns the full slice of incidents.
// When filename is empty the built-in DefaultDataset is used.
func loadDataset(filename string) ([]YAMLIncident, error) {
	var raw []byte
	if filename == "" {
		if len(DefaultDataset) == 0 {
			return nil, fmt.Errorf("no dataset file specified and no built-in dataset available")
		}
		raw = DefaultDataset
	} else {
		var err error
		raw, err = os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("reading dataset: %w", err)
		}
	}
	var ds dataset
	if err := yaml.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("parsing dataset: %w", err)
	}
	return ds.Incidents, nil
}

// pick returns the first count incidents, or all of them when count <= 0.
func pick(all []YAMLIncident, count int) []YAMLIncident {
	if count <= 0 || count >= len(all) {
		return all
	}
	return all[:count]
}

// ─── HTTP helper ─────────────────────────────────────────────────────────────

// postJSON marshals body to JSON, POSTs it to url with the given headers, and
// unmarshals the response body into a map. Non-2xx responses are returned as errors.
func postJSON(url string, headers map[string]string, body any) (map[string]any, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

// stringField safely extracts a string value from a response map.
func stringField(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			s, _ := v.(string)
			return s
		}
		cur, _ = v.(map[string]any)
		if cur == nil {
			return ""
		}
	}
	return ""
}
