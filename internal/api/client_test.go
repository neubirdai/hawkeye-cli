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
