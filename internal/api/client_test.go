package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"hawkeye-cli/internal/config"
)

func TestIsDeltaTrue(t *testing.T) {
	tests := []struct {
		name string
		meta *Metadata
		want bool
	}{
		{"nil metadata", nil, false},
		{"bool true", &Metadata{IsDelta: true}, true},
		{"bool false", &Metadata{IsDelta: false}, false},
		{"string true", &Metadata{IsDelta: "true"}, true},
		{"string false", &Metadata{IsDelta: "false"}, false},
		{"nil value", &Metadata{IsDelta: nil}, false},
		{"number", &Metadata{IsDelta: float64(1)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.meta.IsDeltaTrue()
			if got != tt.want {
				t.Errorf("IsDeltaTrue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewUUID(t *testing.T) {
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	// Generate several UUIDs and check format
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newUUID()
		if !uuidRe.MatchString(id) {
			t.Errorf("newUUID() = %q, does not match UUID v4 format", id)
		}
		if seen[id] {
			t.Errorf("newUUID() returned duplicate: %q", id)
		}
		seen[id] = true
	}
}

func TestSetHeaders(t *testing.T) {
	t.Run("with token and body", func(t *testing.T) {
		c := &Client{token: "my-jwt-token"}
		req, _ := http.NewRequest("POST", "http://example.com", nil)
		c.setHeaders(req, true)

		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		if got := req.Header.Get("Authorization"); got != "Bearer my-jwt-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer my-jwt-token")
		}
	})

	t.Run("without body", func(t *testing.T) {
		c := &Client{token: "tok"}
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		c.setHeaders(req, false)

		if got := req.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want empty for GET", got)
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
	})

	t.Run("no token", func(t *testing.T) {
		c := &Client{}
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		c.setHeaders(req, false)

		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty when no token", got)
		}
	})
}

func TestDoJSON(t *testing.T) {
	t.Run("GET request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"name":"test"}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "test-token"}
		var result struct{ Name string }
		err := c.doJSON("GET", "/test", nil, &result)
		if err != nil {
			t.Fatalf("doJSON() error = %v", err)
		}
		if result.Name != "test" {
			t.Errorf("result.Name = %q, want %q", result.Name, "test")
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("method = %s, want POST", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			var req struct{ Value string }
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if req.Value != "hello" {
				t.Errorf("request body Value = %q, want %q", req.Value, "hello")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
		reqBody := struct{ Value string }{Value: "hello"}
		var result struct{ Ok bool }
		err := c.doJSON("POST", "/test", reqBody, &result)
		if err != nil {
			t.Fatalf("doJSON() error = %v", err)
		}
		if !result.Ok {
			t.Error("result.Ok = false, want true")
		}
	})

	t.Run("error response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, "internal error")
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client()}
		var result struct{}
		err := c.doJSON("GET", "/test", nil, &result)
		if err == nil {
			t.Fatal("doJSON() expected error for 500 response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error = %q, expected to contain status code 500", err.Error())
		}
	})
}

func TestLoginFallback(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// First 3 endpoints fail, 4th succeeds
		if callCount < 4 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, "not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"access_token":"jwt-success"}`)
	}))
	defer srv.Close()

	c := NewClientWithServer(srv.URL)
	c.httpClient = srv.Client()

	resp, err := c.Login("user@test.com", "pass")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if resp.AccessToken != "jwt-success" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "jwt-success")
	}
	if callCount != 4 {
		t.Errorf("tried %d endpoints, want 4", callCount)
	}
}

func TestLoginAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, "not found")
	}))
	defer srv.Close()

	c := NewClientWithServer(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.Login("user@test.com", "pass")
	if err == nil {
		t.Fatal("Login() expected error when all endpoints fail")
	}
	if !strings.Contains(err.Error(), "tried 4 endpoints") {
		t.Errorf("error = %q, expected to mention trying 4 endpoints", err.Error())
	}
}

func TestLoginErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"error":"invalid credentials"}`)
	}))
	defer srv.Close()

	c := NewClientWithServer(srv.URL)
	c.httpClient = srv.Client()

	_, err := c.Login("user@test.com", "wrongpass")
	if err == nil {
		t.Fatal("Login() expected error for error response")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("error = %q, expected to contain error message", err.Error())
	}
}

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		Server:  "http://localhost:3001/",
		Token:   "my-token",
		OrgUUID: "org-123",
	}
	c := NewClient(cfg)
	if c.baseURL != "http://localhost:3001" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
	if c.token != "my-token" {
		t.Errorf("token = %q, want %q", c.token, "my-token")
	}
	if c.orgUUID != "org-123" {
		t.Errorf("orgUUID = %q, want %q", c.orgUUID, "org-123")
	}
}

func TestNormalizeBackendURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple URL", "https://example.com", "https://example.com/api"},
		{"trailing slash", "https://example.com/", "https://example.com/api"},
		{"already has /api", "https://example.com/api", "https://example.com/api"},
		{"already has /api/", "https://example.com/api/", "https://example.com/api"},
		{"localhost", "http://localhost:3000", "http://localhost:3000/api"},
		{"localhost trailing", "http://localhost:3000/", "http://localhost:3000/api"},
		{"neubird URL", "https://copeye.app.neubird.ai", "https://copeye.app.neubird.ai/api"},
		{"neubird URL slash", "https://copeye.app.neubird.ai/", "https://copeye.app.neubird.ai/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeBackendURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeBackendURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessPromptStream(t *testing.T) {
	t.Run("basic SSE parsing", func(t *testing.T) {
		ssePayload := `event: message
data: {"message":{"content":{"content_type":"CONTENT_TYPE_PROGRESS_STATUS","parts":["Working..."]}}}

event: message
data: {"message":{"content":{"content_type":"CONTENT_TYPE_CHAT_RESPONSE","parts":["Hello"]},"end_turn":true}}

`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, ssePayload)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}

		var events []*ProcessPromptResponse
		err := c.ProcessPromptStream("proj", "sess", "test prompt", func(resp *ProcessPromptResponse) {
			events = append(events, resp)
		})
		if err != nil {
			t.Fatalf("ProcessPromptStream() error = %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("got %d events, want 2", len(events))
		}
		if events[0].Message.Content.ContentType != "CONTENT_TYPE_PROGRESS_STATUS" {
			t.Errorf("event[0] content type = %q", events[0].Message.Content.ContentType)
		}
		if events[1].Message.Content.ContentType != "CONTENT_TYPE_CHAT_RESPONSE" {
			t.Errorf("event[1] content type = %q", events[1].Message.Content.ContentType)
		}
		if !events[1].Message.EndTurn {
			t.Error("event[1] EndTurn = false, want true")
		}
	})

	t.Run("gRPC envelope format", func(t *testing.T) {
		// The envelope path is triggered when direct unmarshal fails.
		// Use a type mismatch ("error":123 instead of string) to trigger it.
		ssePayload := `data: {"error":123,"result":{"message":{"content":{"content_type":"CONTENT_TYPE_CHAT_RESPONSE","parts":["wrapped"]},"end_turn":true}}}

`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, ssePayload)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}

		var events []*ProcessPromptResponse
		err := c.ProcessPromptStream("proj", "sess", "test", func(resp *ProcessPromptResponse) {
			events = append(events, resp)
		})
		if err != nil {
			t.Fatalf("ProcessPromptStream() error = %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events[0].Message == nil || events[0].Message.Content == nil {
			t.Fatal("event Message or Content is nil")
		}
		if events[0].Message.Content.Parts[0] != "wrapped" {
			t.Errorf("event part = %q, want %q", events[0].Message.Content.Parts[0], "wrapped")
		}
	})

	t.Run("skips DONE and keepalive", func(t *testing.T) {
		ssePayload := `data: [DONE]
data: :keepalive
data: {"message":{"content":{"content_type":"CONTENT_TYPE_CHAT_RESPONSE","parts":["ok"]},"end_turn":true}}

`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, ssePayload)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}

		var events []*ProcessPromptResponse
		err := c.ProcessPromptStream("proj", "sess", "test", func(resp *ProcessPromptResponse) {
			events = append(events, resp)
		})
		if err != nil {
			t.Fatalf("ProcessPromptStream() error = %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("got %d events, want 1 (DONE and keepalive should be skipped)", len(events))
		}
	})

	t.Run("event type propagation", func(t *testing.T) {
		ssePayload := `event: cot_start
data: {"message":{"content":{"content_type":"CONTENT_TYPE_CHAIN_OF_THOUGHT","parts":["thinking"]}}}

event: message
data: {"message":{"content":{"content_type":"CONTENT_TYPE_CHAT_RESPONSE","parts":["done"]},"end_turn":true}}

`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, ssePayload)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}

		var events []*ProcessPromptResponse
		err := c.ProcessPromptStream("proj", "sess", "test", func(resp *ProcessPromptResponse) {
			events = append(events, resp)
		})
		if err != nil {
			t.Fatalf("ProcessPromptStream() error = %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("got %d events, want 2", len(events))
		}
		if events[0].EventType != "cot_start" {
			t.Errorf("event[0] EventType = %q, want %q", events[0].EventType, "cot_start")
		}
		if events[1].EventType != "message" {
			t.Errorf("event[1] EventType = %q, want %q", events[1].EventType, "message")
		}
	})

	t.Run("HTTP error response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, "unauthorized")
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "bad"}

		err := c.ProcessPromptStream("proj", "sess", "test", func(resp *ProcessPromptResponse) {})
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("error = %q, expected to contain 401", err.Error())
		}
	})
}

func TestSessionListWithFilters(t *testing.T) {
	t.Run("without filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var req SessionListRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if req.Pagination.Limit != 10 {
				t.Errorf("limit = %d, want 10", req.Pagination.Limit)
			}
			if len(req.Filters) != 0 {
				t.Errorf("filters len = %d, want 0", len(req.Filters))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"sessions":[{"session_uuid":"s1","name":"Test"}]}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
		resp, err := c.SessionList("proj", 0, 10, nil)
		if err != nil {
			t.Fatalf("SessionList() error = %v", err)
		}
		if len(resp.Sessions) != 1 {
			t.Errorf("got %d sessions, want 1", len(resp.Sessions))
		}
	})

	t.Run("with filters", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var req SessionListRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(req.Filters) != 2 {
				t.Errorf("filters len = %d, want 2", len(req.Filters))
			}
			if req.Filters[0].Key != "investigation_status" {
				t.Errorf("filter[0].Key = %q, want %q", req.Filters[0].Key, "investigation_status")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"sessions":[]}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
		filters := []PaginationFilter{
			{Key: "investigation_status", Value: "INVESTIGATION_STATUS_COMPLETED", Operator: "=="},
			{Key: "create_time", Value: "2025-01-01", Operator: "gte"},
		}
		resp, err := c.SessionList("proj", 0, 20, filters)
		if err != nil {
			t.Fatalf("SessionList() error = %v", err)
		}
		if len(resp.Sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(resp.Sessions))
		}
	})
}

func TestGetIncidentReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/inference/incident_report") {
			t.Errorf("path = %s, want /v1/inference/incident_report", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"avg_investigation_time_saved_minutes": 15.5,
			"avg_mttr": 30.2,
			"noise_reduction": 45.0,
			"total_incidents": 100,
			"total_investigations": 150,
			"total_investigation_time_saved_hours": 38.75,
			"start_time": "2025-01-01",
			"end_time": "2025-06-30",
			"incident_type_reports": [
				{"incident_type":"alert","priority_reports":[{"priority":"0","total_incidents":50,"avg_time_saved_minutes":10.0}]}
			]
		}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.GetIncidentReport()
	if err != nil {
		t.Fatalf("GetIncidentReport() error = %v", err)
	}
	if resp.TotalIncidents != 100 {
		t.Errorf("TotalIncidents = %d, want 100", resp.TotalIncidents)
	}
	if resp.AvgMTTR != 30.2 {
		t.Errorf("AvgMTTR = %v, want 30.2", resp.AvgMTTR)
	}
	if len(resp.IncidentTypeReports) != 1 {
		t.Fatalf("IncidentTypeReports len = %d, want 1", len(resp.IncidentTypeReports))
	}
	if resp.IncidentTypeReports[0].Type != "alert" {
		t.Errorf("IncidentTypeReports[0].Type = %q, want %q", resp.IncidentTypeReports[0].Type, "alert")
	}
}

func TestListConnections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/connection") {
			t.Errorf("path = %s, want /v1/connection", r.URL.Path)
		}
		if r.URL.Query().Get("project_uuid") != "proj-123" {
			t.Errorf("project_uuid = %q, want %q", r.URL.Query().Get("project_uuid"), "proj-123")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"specs":[
			{"uuid":"c1","name":"Datadog","type":"datadog","sync_state":"SYNCED","training_state":"TRAINED"},
			{"uuid":"c2","name":"Prometheus","type":"prometheus","sync_state":"SYNCING"}
		]}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.ListConnections("proj-123")
	if err != nil {
		t.Fatalf("ListConnections() error = %v", err)
	}
	if len(resp.Specs) != 2 {
		t.Fatalf("got %d connections, want 2", len(resp.Specs))
	}
	if resp.Specs[0].Name != "Datadog" {
		t.Errorf("Specs[0].Name = %q, want %q", resp.Specs[0].Name, "Datadog")
	}
	if resp.Specs[1].SyncState != "SYNCING" {
		t.Errorf("Specs[1].SyncState = %q, want %q", resp.Specs[1].SyncState, "SYNCING")
	}
}

func TestListConnectionResources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/resource") {
			t.Errorf("path = %s, want /v1/resource", r.URL.Path)
		}
		if r.URL.Query().Get("connection_uuid") != "c1" {
			t.Errorf("connection_uuid = %q, want %q", r.URL.Query().Get("connection_uuid"), "c1")
		}
		if r.URL.Query().Get("pagination.limit") != "100" {
			t.Errorf("pagination.limit = %q, want %q", r.URL.Query().Get("pagination.limit"), "100")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"specs":[
			{"id":{"name":"cpu-usage","uuid":"r1"},"connection_uuid":"c1","telemetry_type":"metric"},
			{"id":{"name":"error-logs","uuid":"r2"},"connection_uuid":"c1","telemetry_type":"log"}
		]}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.ListConnectionResources("c1", 100)
	if err != nil {
		t.Fatalf("ListConnectionResources() error = %v", err)
	}
	if len(resp.Specs) != 2 {
		t.Fatalf("got %d resources, want 2", len(resp.Specs))
	}
	if resp.Specs[0].ID.Name != "cpu-usage" {
		t.Errorf("Specs[0].ID.Name = %q, want %q", resp.Specs[0].ID.Name, "cpu-usage")
	}
	if resp.Specs[1].TelemetryType != "log" {
		t.Errorf("Specs[1].TelemetryType = %q, want %q", resp.Specs[1].TelemetryType, "log")
	}
}

// ─── Phase 1: Project CRUD ──────────────────────────────────────────────────

func TestGetProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/gendb/spec/proj-1") {
			t.Errorf("path = %s, want suffix /v1/gendb/spec/proj-1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"spec":{"uuid":"proj-1","name":"My Project","description":"desc","ready":true,"create_time":"2025-01-01"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.GetProject("proj-1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if resp.Spec == nil {
		t.Fatal("Spec is nil")
	}
	if resp.Spec.Name != "My Project" {
		t.Errorf("Name = %q, want %q", resp.Spec.Name, "My Project")
	}
	if !resp.Spec.Ready {
		t.Error("Ready = false, want true")
	}
}

func TestCreateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/gendb/spec" {
			t.Errorf("path = %s, want /v1/gendb/spec", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateProjectRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Name != "New Project" {
			t.Errorf("Name = %q, want %q", req.Name, "New Project")
		}
		if req.Description != "A test project" {
			t.Errorf("Description = %q, want %q", req.Description, "A test project")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"spec":{"uuid":"new-uuid","name":"New Project","description":"A test project"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.CreateProject("New Project", "A test project")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if resp.Spec == nil {
		t.Fatal("Spec is nil")
	}
	if resp.Spec.UUID != "new-uuid" {
		t.Errorf("UUID = %q, want %q", resp.Spec.UUID, "new-uuid")
	}
}

func TestUpdateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/gendb/spec/proj-1") {
			t.Errorf("path = %s, want suffix /v1/gendb/spec/proj-1", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req UpdateProjectRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Name != "Updated" {
			t.Errorf("Name = %q, want %q", req.Name, "Updated")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"spec":{"uuid":"proj-1","name":"Updated"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.UpdateProject("proj-1", "Updated", "new desc")
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if resp.Spec.Name != "Updated" {
		t.Errorf("Name = %q, want %q", resp.Spec.Name, "Updated")
	}
}

func TestDeleteProject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("method = %s, want DELETE", r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/v1/gendb/spec/proj-1") {
				t.Errorf("path = %s, want suffix /v1/gendb/spec/proj-1", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
		if err := c.DeleteProject("proj-1"); err != nil {
			t.Fatalf("DeleteProject() error = %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"response":{"error_code":404,"error_message":"not found"}}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
		err := c.DeleteProject("proj-1")
		if err == nil {
			t.Fatal("expected error for server error response")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
	})
}

// ─── Phase 2: Connections ───────────────────────────────────────────────────

func TestGetConnectionInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/connection/conn-1") {
			t.Errorf("path = %s, want suffix /v1/connection/conn-1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"spec":{"uuid":"conn-1","name":"Datadog Prod","connection_type":"datadog","sync_state":"SYNCED","training_state":"TRAINED"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.GetConnectionInfo("conn-1")
	if err != nil {
		t.Fatalf("GetConnectionInfo() error = %v", err)
	}
	if resp.Spec.Name != "Datadog Prod" {
		t.Errorf("Name = %q, want %q", resp.Spec.Name, "Datadog Prod")
	}
	if resp.Spec.SyncState != "SYNCED" {
		t.Errorf("SyncState = %q, want %q", resp.Spec.SyncState, "SYNCED")
	}
}

func TestCreateConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/datasource/connection" {
			t.Errorf("path = %s, want /v1/datasource/connection", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateConnectionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Name != "My AWS" {
			t.Errorf("Name = %q, want %q", req.Name, "My AWS")
		}
		if req.Type != "aws" {
			t.Errorf("Type = %q, want %q", req.Type, "aws")
		}
		if req.Config["role_arn"] != "arn:aws:iam::123:role/test" {
			t.Errorf("Config[role_arn] = %q", req.Config["role_arn"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"spec":{"uuid":"new-conn","name":"My AWS","type":"aws"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	cfg := map[string]string{"role_arn": "arn:aws:iam::123:role/test"}
	resp, err := c.CreateConnection("My AWS", "aws", cfg)
	if err != nil {
		t.Fatalf("CreateConnection() error = %v", err)
	}
	if resp.Spec.UUID != "new-conn" {
		t.Errorf("UUID = %q, want %q", resp.Spec.UUID, "new-conn")
	}
}

func TestWaitForConnectionSync(t *testing.T) {
	t.Run("already synced", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"spec":{"uuid":"conn-1","sync_state":"SYNCED"}}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
		resp, err := c.WaitForConnectionSync("conn-1", 10)
		if err != nil {
			t.Fatalf("WaitForConnectionSync() error = %v", err)
		}
		if resp.Spec.SyncState != "SYNCED" {
			t.Errorf("SyncState = %q, want SYNCED", resp.Spec.SyncState)
		}
	})

	t.Run("sync failed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"spec":{"uuid":"conn-1","sync_state":"SYNC_STATE_FAILED"}}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
		_, err := c.WaitForConnectionSync("conn-1", 10)
		if err == nil {
			t.Fatal("expected error for failed sync")
		}
		if !strings.Contains(err.Error(), "sync failed") {
			t.Errorf("error = %q, want to contain 'sync failed'", err.Error())
		}
	})
}

func TestAddConnectionToProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/gendb/spec/proj-1/datasource-connections") {
			t.Errorf("path = %s, want to contain /v1/gendb/spec/proj-1/datasource-connections", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req AddConnectionToProjectRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ConnectionUUID != "conn-1" {
			t.Errorf("ConnectionUUID = %q, want %q", req.ConnectionUUID, "conn-1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	if err := c.AddConnectionToProject("proj-1", "conn-1"); err != nil {
		t.Fatalf("AddConnectionToProject() error = %v", err)
	}
}

func TestRemoveConnectionFromProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/gendb/spec/proj-1/datasource-connections") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req RemoveConnectionFromProjectRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ConnectionUUID != "conn-1" {
			t.Errorf("ConnectionUUID = %q, want %q", req.ConnectionUUID, "conn-1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	if err := c.RemoveConnectionFromProject("proj-1", "conn-1"); err != nil {
		t.Fatalf("RemoveConnectionFromProject() error = %v", err)
	}
}

func TestListProjectConnections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/connection") {
			t.Errorf("path = %s, want suffix /v1/connection", r.URL.Path)
		}
		if r.URL.Query().Get("project_uuid") != "proj-1" {
			t.Errorf("project_uuid = %q, want %q", r.URL.Query().Get("project_uuid"), "proj-1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"specs":[{"uuid":"c1","name":"DD","connection_type":"datadog"},{"uuid":"c2","name":"AWS","connection_type":"aws"}]}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.ListProjectConnections("proj-1")
	if err != nil {
		t.Fatalf("ListProjectConnections() error = %v", err)
	}
	if len(resp.Specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(resp.Specs))
	}
	if resp.Specs[0].Name != "DD" {
		t.Errorf("Specs[0].Name = %q, want %q", resp.Specs[0].Name, "DD")
	}
}

// ─── Phase 3: Instructions ──────────────────────────────────────────────────

func TestListInstructions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/instruction" {
			t.Errorf("path = %s, want /v1/instruction", r.URL.Path)
		}
		if r.URL.Query().Get("project_uuid") != "proj-1" {
			t.Errorf("project_uuid = %q, want proj-1", r.URL.Query().Get("project_uuid"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"instructions":[{"uuid":"i1","name":"Filter noise","type":"filter","content":"ignore 404s","enabled":true}]}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.ListInstructions("proj-1")
	if err != nil {
		t.Fatalf("ListInstructions() error = %v", err)
	}
	if len(resp.Instructions) != 1 {
		t.Fatalf("got %d instructions, want 1", len(resp.Instructions))
	}
	if resp.Instructions[0].Name != "Filter noise" {
		t.Errorf("Name = %q, want %q", resp.Instructions[0].Name, "Filter noise")
	}
	if !resp.Instructions[0].Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestCreateInstruction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/instruction" {
			t.Errorf("path = %s, want /v1/instruction", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateInstructionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Instruction.Name != "My rule" {
			t.Errorf("Instruction.Name = %q, want %q", req.Instruction.Name, "My rule")
		}
		if req.Instruction.Type != "filter" {
			t.Errorf("Instruction.Type = %q, want %q", req.Instruction.Type, "filter")
		}
		if req.Instruction.Content != "ignore 404s" {
			t.Errorf("Instruction.Content = %q, want %q", req.Instruction.Content, "ignore 404s")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"instruction":{"uuid":"new-instr","name":"My rule","type":"filter","content":"ignore 404s","enabled":true}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.CreateInstruction("proj-1", "My rule", "filter", "ignore 404s")
	if err != nil {
		t.Fatalf("CreateInstruction() error = %v", err)
	}
	if resp.Instruction.UUID != "new-instr" {
		t.Errorf("UUID = %q, want %q", resp.Instruction.UUID, "new-instr")
	}
}

func TestUpdateInstructionStatus(t *testing.T) {
	t.Run("enable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PUT" {
				t.Errorf("method = %s, want PUT", r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/v1/instruction/instr-1") {
				t.Errorf("path = %s", r.URL.Path)
			}
			body, _ := io.ReadAll(r.Body)
			var req UpdateInstructionStatusRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if req.Status != "INSTRUCTION_STATUS_ENABLED" {
				t.Errorf("Status = %q, want INSTRUCTION_STATUS_ENABLED", req.Status)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
		if err := c.UpdateInstructionStatus("instr-1", true); err != nil {
			t.Fatalf("UpdateInstructionStatus() error = %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"response":{"error_code":403,"error_message":"forbidden"}}`)
		}))
		defer srv.Close()

		c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
		err := c.UpdateInstructionStatus("instr-1", true)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "forbidden") {
			t.Errorf("error = %q, want to contain 'forbidden'", err.Error())
		}
	})
}

func TestDeleteInstruction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/instruction/instr-1") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	if err := c.DeleteInstruction("instr-1"); err != nil {
		t.Fatalf("DeleteInstruction() error = %v", err)
	}
}

func TestValidateInstruction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/instruction/validate" {
			t.Errorf("path = %s, want /v1/instruction/validate", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req ValidateInstructionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Instruction.Type != "filter" {
			t.Errorf("Instruction.Type = %q, want %q", req.Instruction.Type, "filter")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"instruction":{"name":"test-rule","type":"filter","content":"test content"}}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.ValidateInstruction("filter", "test content")
	if err != nil {
		t.Fatalf("ValidateInstruction() error = %v", err)
	}
	if resp.Instruction == nil {
		t.Fatal("Instruction is nil, want non-nil")
	}
	if resp.Instruction.Name != "test-rule" {
		t.Errorf("Name = %q, want %q", resp.Instruction.Name, "test-rule")
	}
}

func TestApplySessionInstruction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1/inference/session/sess-1/instructions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req ApplySessionInstructionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Type != "system" {
			t.Errorf("Type = %q, want %q", req.Type, "system")
		}
		if req.Content != "focus on RCA" {
			t.Errorf("Content = %q, want %q", req.Content, "focus on RCA")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	if err := c.ApplySessionInstruction("sess-1", "system", "focus on RCA"); err != nil {
		t.Fatalf("ApplySessionInstruction() error = %v", err)
	}
}

func TestRerunSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/inference/session/sess-1:rerun") {
			t.Errorf("path = %s, want suffix /v1/inference/session/sess-1:rerun", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"session_uuid":"sess-1-rerun"}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.RerunSession("sess-1")
	if err != nil {
		t.Fatalf("RerunSession() error = %v", err)
	}
	if resp.SessionUUID != "sess-1-rerun" {
		t.Errorf("SessionUUID = %q, want %q", resp.SessionUUID, "sess-1-rerun")
	}
}

// ─── Phase 4: Investigation Enhancements ────────────────────────────────────

func TestCreateSessionFromAlert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/inference/session:create" {
			t.Errorf("path = %s, want /v1/inference/session:create", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateSessionFromAlertRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ProjectUUID != "proj-1" {
			t.Errorf("ProjectUUID = %q, want %q", req.ProjectUUID, "proj-1")
		}
		if req.AlertID != "alert-42" {
			t.Errorf("AlertID = %q, want %q", req.AlertID, "alert-42")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"session_uuid":"alert-sess-uuid"}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.CreateSessionFromAlert("proj-1", "alert-42")
	if err != nil {
		t.Fatalf("CreateSessionFromAlert() error = %v", err)
	}
	if resp.SessionUUID != "alert-sess-uuid" {
		t.Errorf("SessionUUID = %q, want %q", resp.SessionUUID, "alert-sess-uuid")
	}
}

func TestGetInvestigationQueries(t *testing.T) {
	// GetInvestigationQueries now internally calls SessionInspect and extracts queries from chain_of_thoughts.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/inference/session/inspect" {
			t.Errorf("path = %s, want /v1/inference/session/inspect", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"prompt_cycle":[{"chain_of_thoughts":[
			{"id":"q1","description":"Check error logs","status":"done","sources_involved":["log_source"]},
			{"id":"q2","description":"Check metrics","status":"done","sources_involved":[]}
		]}]}`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok", orgUUID: "org"}
	resp, err := c.GetInvestigationQueries("proj-1", "sess-1")
	if err != nil {
		t.Fatalf("GetInvestigationQueries() error = %v", err)
	}
	if len(resp.Queries) != 2 {
		t.Fatalf("got %d queries, want 2", len(resp.Queries))
	}
	if resp.Queries[0].Query != "Check error logs" {
		t.Errorf("Queries[0].Query = %q, want %q", resp.Queries[0].Query, "Check error logs")
	}
	if resp.Queries[0].Source != "log_source" {
		t.Errorf("Queries[0].Source = %q, want %q", resp.Queries[0].Source, "log_source")
	}
}

// ─── Phase 5: Discovery & Reports ───────────────────────────────────────────

func TestDiscoverProjectResources(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		// First call: ListProjectConnections
		if r.URL.Path == "/v1/connection" && r.URL.Query().Get("project_uuid") != "" {
			_, _ = fmt.Fprint(w, `{"specs":[{"uuid":"c1","name":"DD","connection_type":"datadog"},{"uuid":"c2","name":"AWS","connection_type":"aws"}]}`)
			return
		}
		// Subsequent calls: ListConnectionResources for each connection
		if strings.Contains(r.URL.Path, "/v1/resource") {
			connUUID := r.URL.Query().Get("connection_uuid")
			if connUUID == "c1" {
				_, _ = fmt.Fprint(w, `{"specs":[{"id":{"name":"cpu","uuid":"r1"},"connection_uuid":"c1","telemetry_type":"metric"}]}`)
			} else {
				_, _ = fmt.Fprint(w, `{"specs":[{"id":{"name":"logs","uuid":"r2"},"connection_uuid":"c2","telemetry_type":"log"}]}`)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.DiscoverProjectResources("proj-1", "", "")
	if err != nil {
		t.Fatalf("DiscoverProjectResources() error = %v", err)
	}
	if len(resp.Resources) != 2 {
		t.Fatalf("got %d resources, want 2", len(resp.Resources))
	}

	// Test filtering by telemetry type
	resp2, err := c.DiscoverProjectResources("proj-1", "metric", "")
	if err != nil {
		t.Fatalf("DiscoverProjectResources(metric) error = %v", err)
	}
	if len(resp2.Resources) != 1 {
		t.Fatalf("filtered got %d resources, want 1", len(resp2.Resources))
	}
	if resp2.Resources[0].ID.Name != "cpu" {
		t.Errorf("filtered resource = %q, want %q", resp2.Resources[0].ID.Name, "cpu")
	}
}

func TestDiscoverProjectResourcesFilterByConnectionType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/connection" && r.URL.Query().Get("project_uuid") != "" {
			_, _ = fmt.Fprint(w, `{"specs":[{"uuid":"c1","name":"DD","connection_type":"datadog"},{"uuid":"c2","name":"AWS","connection_type":"aws"}]}`)
			return
		}
		if strings.Contains(r.URL.Path, "/v1/resource") {
			connUUID := r.URL.Query().Get("connection_uuid")
			if connUUID == "c1" {
				_, _ = fmt.Fprint(w, `{"specs":[{"id":{"name":"cpu","uuid":"r1"},"connection_uuid":"c1","telemetry_type":"metric"}]}`)
			} else {
				_, _ = fmt.Fprint(w, `{"specs":[{"id":{"name":"logs","uuid":"r2"},"connection_uuid":"c2","telemetry_type":"log"}]}`)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	resp, err := c.DiscoverProjectResources("proj-1", "", "aws")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(resp.Resources) != 1 {
		t.Fatalf("got %d resources, want 1 (aws only)", len(resp.Resources))
	}
	if resp.Resources[0].ID.Name != "logs" {
		t.Errorf("resource = %q, want %q", resp.Resources[0].ID.Name, "logs")
	}
}

func TestGetSessionReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/inference/session_report" {
			t.Errorf("path = %s, want /v1/inference/session_report", r.URL.Path)
		}
		if r.URL.Query().Get("project_uuid") != "proj-1" {
			t.Errorf("project_uuid = %q", r.URL.Query().Get("project_uuid"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `[{"summary":"Root cause: memory leak","time_saved":1500,"session_link":"https://example.com/sess-1"}]`)
	}))
	defer srv.Close()

	c := &Client{baseURL: srv.URL, httpClient: srv.Client(), token: "tok"}
	items, err := c.GetSessionReport("proj-1", []string{"sess-1"})
	if err != nil {
		t.Fatalf("GetSessionReport() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Summary != "Root cause: memory leak" {
		t.Errorf("Summary = %q", items[0].Summary)
	}
	if items[0].TimeSaved != 1500 {
		t.Errorf("TimeSaved = %d, want 1500", items[0].TimeSaved)
	}
}

// Verify *Client implements HawkeyeAPI at compile time.
var _ HawkeyeAPI = (*Client)(nil)
