package api

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"hawkeye-cli/internal/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	orgUUID    string
	debug      bool
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.Server, "/"),
		httpClient: &http.Client{
			// No timeout on the client — investigations can take 30+ minutes.
			// We rely on the server closing the SSE stream (end_turn) to finish.
			Timeout: 0,
		},
		token:   cfg.Token,
		orgUUID: cfg.OrgUUID,
	}
}

// SetDebug enables debug output for SSE parsing.
func (c *Client) SetDebug(on bool) { c.debug = on }

func (c *Client) setHeaders(req *http.Request, hasBody bool) {
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// --- Authentication ---

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	Token        string `json:"token,omitempty"`
	OrgUUID      string `json:"org_uuid,omitempty"`
	UserUUID     string `json:"user_uuid,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Error        string `json:"error,omitempty"`
}

type UserSpec struct {
	UUID      string `json:"uuid"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	OrgUUID   string `json:"org_uuid"`
	UserRole  string `json:"user_role"`
}

type UserInfoResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Specs    []UserSpec     `json:"specs,omitempty"`
}

// NewClientWithServer creates a client from just a server URL (for login before config is set).
func NewClientWithServer(server string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(server, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func ResolveBackendURL(frontendURL string) (string, error) {
	frontendURL = strings.TrimRight(frontendURL, "/")
	envURL := frontendURL + "/env.js"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(envURL)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", envURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: status %d", envURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", envURL, err)
	}

	content := string(body)
	key := "VITE_BASE_API_URL:"
	idx := strings.Index(content, key)
	if idx < 0 {
		return "", fmt.Errorf("VITE_BASE_API_URL not found in %s", envURL)
	}

	after := content[idx+len(key):]
	quote := byte('"')
	start := strings.IndexByte(after, quote)
	if start < 0 {
		quote = '\''
		start = strings.IndexByte(after, quote)
	}
	if start < 0 {
		return "", fmt.Errorf("could not parse VITE_BASE_API_URL value")
	}
	end := strings.IndexByte(after[start+1:], quote)
	if end < 0 {
		return "", fmt.Errorf("could not parse VITE_BASE_API_URL value")
	}

	return strings.TrimRight(after[start+1:start+1+end], "/"), nil
}

func (c *Client) Login(email, password string) (*LoginResponse, error) {
	reqBody := LoginRequest{Email: email, Password: password}

	endpoints := []string{
		"/v1/user/login",
		"/v1/auth/login",
		"/api/v1/login",
		"/login",
	}

	var lastErr error
	for _, ep := range endpoints {
		var resp LoginResponse
		err := c.doJSON("POST", ep, reqBody, &resp)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.Error != "" {
			return nil, fmt.Errorf("login failed: %s", resp.Error)
		}
		if resp.ErrorMessage != "" {
			return nil, fmt.Errorf("login failed: %s", resp.ErrorMessage)
		}

		token := resp.AccessToken
		if token == "" {
			token = resp.Token
		}
		if token == "" {
			lastErr = fmt.Errorf("no token in response from %s", ep)
			continue
		}

		resp.AccessToken = token
		c.token = token
		return &resp, nil
	}

	return nil, fmt.Errorf("login failed (tried %d endpoints): %w", len(endpoints), lastErr)
}

func (c *Client) FetchUserInfo() (*UserSpec, error) {
	var resp UserInfoResponse
	if err := c.doJSON("GET", "/v1/user", nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Specs) == 0 {
		return nil, fmt.Errorf("no user info returned")
	}
	return &resp.Specs[0], nil
}

// --- Session Management ---

type GenDBRequest struct {
	RequestID        string `json:"request_id,omitempty"`
	ClientIdentifier string `json:"client_identifier,omitempty"`
	UUID             string `json:"uuid,omitempty"`
}

type GenDBSpec struct {
	UUID string `json:"uuid,omitempty"`
}

type NewSessionRequest struct {
	Request          *GenDBRequest `json:"request,omitempty"`
	OrganizationUUID string        `json:"organization_uuid,omitempty"`
	ProjectUUID      string        `json:"project_uuid,omitempty"`
	GenDBSpec        *GenDBSpec    `json:"gendb_spec,omitempty"`
}

type GenDBResponse struct {
	RequestID    string `json:"request_id,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ErrorCode    int    `json:"error_code,omitempty"`
	UUID         string `json:"uuid,omitempty"`
}

type NewSessionResponse struct {
	Response    *GenDBResponse `json:"response,omitempty"`
	SessionUUID string         `json:"session_uuid,omitempty"`
}

func (c *Client) NewSession(projectUUID string) (*NewSessionResponse, error) {
	reqBody := NewSessionRequest{
		Request:          &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		OrganizationUUID: c.orgUUID,
		ProjectUUID:      projectUUID,
		GenDBSpec:        &GenDBSpec{UUID: newUUID()},
	}
	var resp NewSessionResponse
	if err := c.doJSON("POST", "/v1/inference/new_session", reqBody, &resp); err != nil {
		return nil, err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return nil, fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return &resp, nil
}

// newUUID generates a random v4 UUID (no external dependencies).
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// --- Process Prompt (Streaming) ---

// Metadata holds per-event metadata from the SSE payload.
type Metadata struct {
	IsDelta interface{} `json:"is_delta,omitempty"` // can be bool or string "true"
}

// IsDeltaTrue returns whether the metadata indicates a delta event.
func (m *Metadata) IsDeltaTrue() bool {
	if m == nil {
		return false
	}
	switch v := m.IsDelta.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

type Message struct {
	ID       string    `json:"id,omitempty"`
	Content  *Content  `json:"content,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
	Status   string    `json:"status,omitempty"`
	EndTurn  bool      `json:"end_turn,omitempty"`
}

type Content struct {
	ContentType string   `json:"content_type,omitempty"`
	Parts       []string `json:"parts,omitempty"`
}

type ProcessPromptRequest struct {
	Request       *GenDBRequest  `json:"request,omitempty"`
	Action        string         `json:"action,omitempty"`
	SessionUUID   string         `json:"session_uuid,omitempty"`
	ProjectUUID   string         `json:"project_uuid,omitempty"`
	PromptOptions *PromptOptions `json:"prompt_options,omitempty"`
	Messages      []Message      `json:"messages,omitempty"`
}

type PromptOptions struct {
	DisableReplay bool `json:"disable_replay,omitempty"`
}

type ProcessPromptResponse struct {
	Response    *GenDBResponse `json:"response,omitempty"`
	Message     *Message       `json:"message,omitempty"`
	SessionUUID string         `json:"session_uuid,omitempty"`
	Error       string         `json:"error,omitempty"`

	// EventType is populated by the SSE parser from the "event:" field.
	// Not part of JSON — set after parsing. Examples: "message", "cot_start",
	// "cot_delta", "cot_end", "prompt_cycle_start", "prompt_cycle_end".
	EventType string `json:"-"`
}

// StreamCallback is called for each streamed response chunk.
type StreamCallback func(resp *ProcessPromptResponse)

func (c *Client) ProcessPromptStream(projectUUID, sessionUUID, prompt string, cb StreamCallback) error {
	reqBody := ProcessPromptRequest{
		Request:     &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		Action:      "ACTION_NEXT",
		SessionUUID: sessionUUID,
		ProjectUUID: projectUUID,
		Messages: []Message{
			{
				Content: &Content{
					ContentType: "CONTENT_TYPE_CHAT_PROMPT",
					Parts:       []string{prompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/inference/session", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(req, true)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(errBody))
	}

	if c.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Content-Type: %s\n", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	// 1 MB buffer for large streamed chunks (chain-of-thought can be huge)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// Track the current SSE event type across lines.
	// SSE format: "event: <type>" line followed by "data: <json>" line,
	// then a blank line separator.
	currentEventType := "message" // default per SSE spec

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// Blank line = end of SSE event block.
			// Reset event type to default for next event.
			currentEventType = "message"
			continue
		}

		// Capture SSE event type
		if strings.HasPrefix(trimmed, "event:") {
			currentEventType = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			if c.debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] event: %s\n", currentEventType)
			}
			continue
		}

		// Skip SSE comments and id/retry fields
		if strings.HasPrefix(trimmed, ":") || strings.HasPrefix(trimmed, "id:") || strings.HasPrefix(trimmed, "retry:") {
			continue
		}

		// Only process data lines
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}

		// Strip SSE "data: " prefix
		var jsonStr string
		if strings.HasPrefix(trimmed, "data: ") {
			jsonStr = strings.TrimPrefix(trimmed, "data: ")
		} else {
			jsonStr = strings.TrimPrefix(trimmed, "data:")
		}

		jsonStr = strings.TrimSpace(jsonStr)
		if jsonStr == "" || jsonStr == "[DONE]" || jsonStr == ":keepalive" {
			continue
		}

		var streamResp ProcessPromptResponse
		if err := json.Unmarshal([]byte(jsonStr), &streamResp); err != nil {
			// Try gRPC-gateway envelope
			var envelope struct {
				Result *ProcessPromptResponse `json:"result"`
			}
			if err2 := json.Unmarshal([]byte(jsonStr), &envelope); err2 == nil && envelope.Result != nil {
				envelope.Result.EventType = currentEventType
				cb(envelope.Result)
				if c.debug && envelope.Result.Message != nil && envelope.Result.Message.Content != nil {
					c.debugLog(currentEventType, envelope.Result)
				}
				if envelope.Result.Message != nil && envelope.Result.Message.EndTurn {
					return nil
				}
				continue
			}
			// Skip unparseable lines
			if c.debug {
				snippet := jsonStr
				if len(snippet) > 80 {
					snippet = snippet[:80] + "..."
				}
				fmt.Fprintf(os.Stderr, "[DEBUG] unparseable: %s\n", snippet)
			}
			continue
		}

		streamResp.EventType = currentEventType
		cb(&streamResp)
		if c.debug && streamResp.Message != nil && streamResp.Message.Content != nil {
			c.debugLog(currentEventType, &streamResp)
		}
		if streamResp.Message != nil && streamResp.Message.EndTurn {
			return nil
		}
	}

	return scanner.Err()
}

// debugLog prints a compact debug line for an SSE event.
func (c *Client) debugLog(eventType string, resp *ProcessPromptResponse) {
	ct := resp.Message.Content.ContentType
	isDelta := ""
	if resp.Message.Metadata != nil && resp.Message.Metadata.IsDeltaTrue() {
		isDelta = " [delta]"
	}
	partsInfo := ""
	if n := len(resp.Message.Content.Parts); n > 1 {
		partsInfo = fmt.Sprintf(" [%d parts]", n)
	}
	partSnippet := ""
	if len(resp.Message.Content.Parts) > 0 {
		p := resp.Message.Content.Parts[0]
		if len(p) > 120 {
			p = p[:120] + "..."
		}
		partSnippet = p
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] evt=%-16s ct=%-40s%s%s | %s\n", eventType, ct, isDelta, partsInfo, partSnippet)
}

// --- Session List ---

type SessionInfo struct {
	SessionUUID         string `json:"session_uuid"`
	Name                string `json:"name"`
	CreateTime          string `json:"create_time"`
	LastUpdate          string `json:"last_update"`
	ProjectUUID         string `json:"project_uuid"`
	SessionType         string `json:"session_type"`
	InvestigationStatus string `json:"investigation_status"`
	Pinned              bool   `json:"pinned"`
}

type PaginationRequest struct {
	Start int `json:"start"`
	Limit int `json:"limit"`
}

type SessionListRequest struct {
	Request          *GenDBRequest      `json:"request,omitempty"`
	Pagination       *PaginationRequest `json:"pagination,omitempty"`
	OrganizationUUID string             `json:"organization_uuid,omitempty"`
	ProjectUUID      string             `json:"project_uuid,omitempty"`
}

type SessionListResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Sessions []SessionInfo  `json:"sessions,omitempty"`
}

func (c *Client) SessionList(projectUUID string, limit int) (*SessionListResponse, error) {
	reqBody := SessionListRequest{
		Request:          &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		OrganizationUUID: c.orgUUID,
		ProjectUUID:      projectUUID,
		Pagination:       &PaginationRequest{Start: 0, Limit: limit},
	}
	var resp SessionListResponse
	if err := c.doJSON("POST", "/v1/inference/session/list", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Session Inspect ---

type PromptCycle struct {
	ID                  string                `json:"id"`
	CreateTime          string                `json:"create_time"`
	FinalAnswer         string                `json:"final_answer"`
	Rating              string                `json:"rating"`
	FollowUpSuggestions []string              `json:"follow_up_suggestions"`
	Sources             []Source              `json:"sources"`
	ChainOfThoughts     []ChainOfThought      `json:"chain_of_thoughts"`
	Status              string                `json:"status"`
	Request             *ProcessPromptRequest `json:"request,omitempty"`
}

type Source struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type ChainOfThought struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Description    string   `json:"description"`
	Status         string   `json:"status"`
	Investigation  string   `json:"investigation"`
	Explanation    string   `json:"explanation"`
	Sources        []string `json:"sources_involved"`
	CotStatus      string   `json:"cot_status"`
	ProcessingTime string   `json:"processing_time"`
}

type SessionInspectRequest struct {
	Request          *GenDBRequest      `json:"request,omitempty"`
	OrganizationUUID string             `json:"organization_uuid,omitempty"`
	ProjectUUID      string             `json:"project_uuid,omitempty"`
	SessionUUID      string             `json:"session_uuid,omitempty"`
	Pagination       *PaginationRequest `json:"pagination,omitempty"`
}

type SessionInspectResponse struct {
	Response    *GenDBResponse `json:"response,omitempty"`
	SessionInfo *SessionInfo   `json:"session_info,omitempty"`
	PromptCycle []PromptCycle  `json:"prompt_cycle,omitempty"`
}

func (c *Client) SessionInspect(projectUUID, sessionUUID string) (*SessionInspectResponse, error) {
	reqBody := SessionInspectRequest{
		Request:          &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		OrganizationUUID: c.orgUUID,
		ProjectUUID:      projectUUID,
		SessionUUID:      sessionUUID,
		Pagination:       &PaginationRequest{Start: 0, Limit: 50},
	}
	var resp SessionInspectResponse
	if err := c.doJSON("POST", "/v1/inference/session/inspect", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Session Summary ---

type SessionSummary struct {
	ActionItems  []string             `json:"action_items"`
	Analysis     string               `json:"analysis"`
	Rating       string               `json:"rating"`
	ShortSummary *ShortSessionSummary `json:"short_session_summary"`
}

type ShortSessionSummary struct {
	Question string `json:"question"`
	Analysis string `json:"analysis"`
}

type GetSessionSummaryResponse struct {
	Response       *GenDBResponse  `json:"response,omitempty"`
	SessionInfo    *SessionInfo    `json:"session_info,omitempty"`
	SessionSummary *SessionSummary `json:"session_summary,omitempty"`
}

func (c *Client) GetSessionSummary(projectUUID, sessionUUID string) (*GetSessionSummaryResponse, error) {
	params := url.Values{}
	params.Set("project_uuid", projectUUID)
	var resp GetSessionSummaryResponse
	if err := c.doJSON("GET", "/v1/inference/session/summary/"+sessionUUID+"?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Projects ---

type ProjectSpec struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

type ListProjectResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Specs    []ProjectSpec  `json:"specs,omitempty"`
}

func (c *Client) ListProjects() (*ListProjectResponse, error) {
	var resp ListProjectResponse
	if err := c.doJSON("GET", "/v1/project", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Prompt Library ---

type InitialPrompt struct {
	UUID     string `json:"uuid"`
	Oneliner string `json:"oneliner"`
	Prompt   string `json:"prompt"`
}

type PromptLibraryResponse struct {
	Response *GenDBResponse  `json:"response,omitempty"`
	Items    []InitialPrompt `json:"items,omitempty"`
}

func (c *Client) PromptLibrary(projectUUID string) (*PromptLibraryResponse, error) {
	params := url.Values{}
	params.Set("project_uuid", projectUUID)
	var resp PromptLibraryResponse
	if err := c.doJSON("GET", "/v1/inference/prompt-library?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Rating / Feedback ---

type RatingItemID struct {
	ItemType string `json:"item_type"`
	ItemID   string `json:"item_id"`
}

type PutRatingRequest struct {
	Request     *GenDBRequest  `json:"request,omitempty"`
	ProjectUUID string         `json:"project_uuid,omitempty"`
	SessionUUID string         `json:"session_uuid,omitempty"`
	ItemIDs     []RatingItemID `json:"item_ids,omitempty"`
	Rating      string         `json:"rating"`
	Reason      string         `json:"rating_reason"`
}

type PutRatingResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
}

func (c *Client) PutRating(projectUUID, sessionUUID string, itemIDs []RatingItemID, rating, reason string) error {
	reqBody := PutRatingRequest{
		Request:     &GenDBRequest{ClientIdentifier: "hawkeye-cli", RequestID: newUUID()},
		ProjectUUID: projectUUID,
		SessionUUID: sessionUUID,
		ItemIDs:     itemIDs,
		Rating:      rating,
		Reason:      reason,
	}
	var resp PutRatingResponse
	if err := c.doJSON("PUT", "/v1/inference/rating", reqBody, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

// --- Generic JSON helper ---

func (c *Client) doJSON(method, path string, reqBody interface{}, result interface{}) error {
	var bodyReader io.Reader
	if reqBody != nil && method != "GET" {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	c.setHeaders(req, bodyReader != nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
	}
	return nil
}
