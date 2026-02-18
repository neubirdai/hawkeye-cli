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

type Message struct {
	ID      string   `json:"id,omitempty"`
	Content *Content `json:"content,omitempty"`
	Status  string   `json:"status,omitempty"`
	EndTurn bool     `json:"end_turn,omitempty"`
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
		fmt.Fprintf(io.Discard, "[DEBUG] Content-Type: %s\n", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	// 1 MB buffer for large streamed chunks (chain-of-thought can be huge)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: lines starting with "data: " contain the JSON payload.
		// Blank lines are event separators — skip them.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Strip SSE "data: " prefix
		jsonStr := trimmed
		if strings.HasPrefix(trimmed, "data: ") {
			jsonStr = strings.TrimPrefix(trimmed, "data: ")
		} else if strings.HasPrefix(trimmed, "data:") {
			jsonStr = strings.TrimPrefix(trimmed, "data:")
		}

		// Skip SSE comments and other non-data fields
		if strings.HasPrefix(trimmed, ":") || strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "id:") || strings.HasPrefix(trimmed, "retry:") {
			continue
		}

		jsonStr = strings.TrimSpace(jsonStr)
		if jsonStr == "" || jsonStr == "[DONE]" {
			continue
		}

		var streamResp ProcessPromptResponse
		if err := json.Unmarshal([]byte(jsonStr), &streamResp); err != nil {
			// Try gRPC-gateway envelope
			var envelope struct {
				Result *ProcessPromptResponse `json:"result"`
			}
			if err2 := json.Unmarshal([]byte(jsonStr), &envelope); err2 == nil && envelope.Result != nil {
				cb(envelope.Result)
				if envelope.Result.Message != nil && envelope.Result.Message.EndTurn {
					return nil
				}
				continue
			}
			// Skip unparseable lines
			continue
		}

		cb(&streamResp)
		if streamResp.Message != nil && streamResp.Message.EndTurn {
			return nil
		}
	}

	return scanner.Err()
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
	ID                  string           `json:"id"`
	CreateTime          string           `json:"create_time"`
	FinalAnswer         string           `json:"final_answer"`
	Rating              string           `json:"rating"`
	FollowUpSuggestions []string         `json:"follow_up_suggestions"`
	Sources             []Source         `json:"sources"`
	ChainOfThoughts     []ChainOfThought `json:"chain_of_thoughts"`
	Status              string           `json:"status"`
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
