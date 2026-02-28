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

// NormalizeBackendURL normalizes a URL to be used as the backend API endpoint.
// It strips trailing slashes and removes any /api suffix to get the base URL,
// then appends /api to ensure a consistent backend endpoint.
func NormalizeBackendURL(inputURL string) string {
	inputURL = strings.TrimRight(inputURL, "/")

	// If URL already ends with /api, use it as-is
	if strings.HasSuffix(inputURL, "/api") {
		return inputURL
	}

	// Otherwise, append /api
	return inputURL + "/api"
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

// PaginationFilter represents a filter for paginated queries.
type PaginationFilter struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Operator string `json:"operator"`
}

// PaginationSort specifies sort order for paginated queries.
type PaginationSort struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending"`
}

type SessionListRequest struct {
	Request          *GenDBRequest      `json:"request,omitempty"`
	Pagination       *PaginationRequest `json:"pagination,omitempty"`
	OrganizationUUID string             `json:"organization_uuid,omitempty"`
	ProjectUUID      string             `json:"project_uuid,omitempty"`
	Filters          []PaginationFilter `json:"filters,omitempty"`
	Sort             []PaginationSort   `json:"sort,omitempty"`
}

type SessionListResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Sessions []SessionInfo  `json:"sessions,omitempty"`
}

func (c *Client) SessionList(projectUUID string, start, limit int, filters []PaginationFilter) (*SessionListResponse, error) {
	reqBody := SessionListRequest{
		Request:          &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		OrganizationUUID: c.orgUUID,
		ProjectUUID:      projectUUID,
		Pagination:       &PaginationRequest{Start: start, Limit: limit},
		Filters:          filters,
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

// ScoreSection holds a numeric score with summary.
type ScoreSection struct {
	Score   float64 `json:"score"`
	Summary string  `json:"summary"`
}

// QualSection holds qualitative assessment data.
type QualSection struct {
	Strengths    []string `json:"strengths"`
	Improvements []string `json:"improvements"`
}

// AnalysisScore holds RCA quality scores.
type AnalysisScore struct {
	Accuracy     ScoreSection `json:"accuracy"`
	Completeness ScoreSection `json:"completeness"`
	Qualitative  QualSection  `json:"qualitative"`
	ScoredBy     string       `json:"scored_by"`
}

// TimeSavedSummary holds time-saved metrics for a session.
type TimeSavedSummary struct {
	TimeSavedMinutes         float64 `json:"time_saved_minutes"`
	StandardInvestigationMin float64 `json:"standard_investigation_time_minutes"`
	HawkeyeInvestigationMin  float64 `json:"hawkeye_investigation_time_minutes"`
}

type SessionSummary struct {
	ActionItems   []string             `json:"action_items"`
	Analysis      string               `json:"analysis"`
	Rating        string               `json:"rating"`
	ShortSummary  *ShortSessionSummary `json:"short_session_summary"`
	AnalysisScore *AnalysisScore       `json:"analysis_score,omitempty"`
	TimeSaved     *TimeSavedSummary    `json:"time_saved,omitempty"`
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

// --- Project CRUD ---

// ProjectDetail holds extended project info from GET /v1/gendb/spec/{uuid}.
type ProjectDetail struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Ready       bool   `json:"ready"`
	CreateTime  string `json:"create_time,omitempty"`
	UpdateTime  string `json:"update_time,omitempty"`
}

// GetProjectResponse holds the response from GET /v1/gendb/spec/{uuid}.
type GetProjectResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Spec     *ProjectDetail `json:"spec,omitempty"`
}

func (c *Client) GetProject(projectUUID string) (*GetProjectResponse, error) {
	var resp GetProjectResponse
	if err := c.doJSON("GET", "/v1/gendb/spec/"+projectUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProjectRequest holds the body for POST /v1/gendb/spec.
type CreateProjectRequest struct {
	Request     *GenDBRequest `json:"request,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
}

// CreateProjectResponse holds the response from POST /v1/gendb/spec.
type CreateProjectResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Spec     *ProjectDetail `json:"spec,omitempty"`
}

func (c *Client) CreateProject(name, description string) (*CreateProjectResponse, error) {
	reqBody := CreateProjectRequest{
		Request:     &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		Name:        name,
		Description: description,
	}
	var resp CreateProjectResponse
	if err := c.doJSON("POST", "/v1/gendb/spec", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateProjectRequest holds the body for PATCH /v1/gendb/spec/{uuid}.
type UpdateProjectRequest struct {
	Request     *GenDBRequest `json:"request,omitempty"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
}

// UpdateProjectResponse holds the response from PATCH /v1/gendb/spec/{uuid}.
type UpdateProjectResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
	Spec     *ProjectDetail `json:"spec,omitempty"`
}

func (c *Client) UpdateProject(projectUUID, name, description string) (*UpdateProjectResponse, error) {
	reqBody := UpdateProjectRequest{
		Request:     &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		Name:        name,
		Description: description,
	}
	var resp UpdateProjectResponse
	if err := c.doJSON("PATCH", "/v1/gendb/spec/"+projectUUID, reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteProjectResponse holds the response from DELETE /v1/gendb/spec/{uuid}.
type DeleteProjectResponse struct {
	Response *GenDBResponse `json:"response,omitempty"`
}

func (c *Client) DeleteProject(projectUUID string) error {
	var resp DeleteProjectResponse
	if err := c.doJSON("DELETE", "/v1/gendb/spec/"+projectUUID, nil, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
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

// --- Incident Report ---

// IncidentTypeReport holds per-type analytics.
// PriorityReport holds metrics for a single priority level within an incident type.
type PriorityReport struct {
	Priority              string  `json:"priority"`
	TotalIncidents        int     `json:"total_incidents"`
	InvestigatedIncidents int     `json:"investigated_incidents"`
	PercentGrouped        float64 `json:"percent_grouped"`
	AvgMTTR               float64 `json:"avg_mttr"`
	AvgInvestigationTime  float64 `json:"avg_investigation_time_minutes"`
	AvgTimeSavedMinutes   float64 `json:"avg_time_saved_minutes"`
}

type IncidentTypeReport struct {
	Type            string           `json:"incident_type"`
	PriorityReports []PriorityReport `json:"priority_reports"`
}

// IncidentReportResponse holds org-wide incident analytics.
type IncidentReportResponse struct {
	AvgTimeSavedMinutes float64              `json:"avg_investigation_time_saved_minutes"`
	AvgMTTR             float64              `json:"avg_mttr"`
	NoiseReduction      float64              `json:"noise_reduction"`
	TotalIncidents      int                  `json:"total_incidents"`
	TotalInvestigations int                  `json:"total_investigations"`
	TotalTimeSavedHours float64              `json:"total_investigation_time_saved_hours"`
	StartTime           string               `json:"start_time"`
	EndTime             string               `json:"end_time"`
	IncidentTypeReports []IncidentTypeReport `json:"incident_type_reports"`
}

func (c *Client) GetIncidentReport() (*IncidentReportResponse, error) {
	var resp IncidentReportResponse
	if err := c.doJSON("GET", "/v1/inference/incident_report", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Connections ---

// ConnectionSpec describes a data source connection.
type ConnectionSpec struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Type          string `json:"connection_type"`
	SyncState     string `json:"sync_state"`
	TrainingState string `json:"training_state"`
}

// ListConnectionsResponse holds the list of connections.
type ListConnectionsResponse struct {
	Specs []ConnectionSpec `json:"specs"`
}

func (c *Client) ListConnections(projectUUID string) (*ListConnectionsResponse, error) {
	params := url.Values{}
	params.Set("project_uuid", projectUUID)
	var resp ListConnectionsResponse
	if err := c.doJSON("GET", "/v1/connection?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Connection CRUD ---

// ConnectionDetail holds extended connection info.
type ConnectionDetail struct {
	UUID          string            `json:"uuid"`
	Name          string            `json:"name"`
	Type          string            `json:"connection_type"`
	SyncState     string            `json:"sync_state"`
	TrainingState string            `json:"training_state"`
	CreateTime    string            `json:"create_time,omitempty"`
	UpdateTime    string            `json:"update_time,omitempty"`
	Config        map[string]string `json:"config,omitempty"`
	Description   string            `json:"description,omitempty"`
}

// GetConnectionResponse holds the response from GET /v1/datasource/connection/{uuid}.
type GetConnectionResponse struct {
	Response *GenDBResponse    `json:"response,omitempty"`
	Spec     *ConnectionDetail `json:"spec,omitempty"`
}

func (c *Client) GetConnectionInfo(connUUID string) (*GetConnectionResponse, error) {
	var resp GetConnectionResponse
	if err := c.doJSON("GET", "/v1/connection/"+connUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateConnectionRequest holds the body for POST /v1/datasource/connection.
type CreateConnectionRequest struct {
	Request *GenDBRequest     `json:"request,omitempty"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config,omitempty"`
}

// CreateConnectionResponse holds the response from POST /v1/datasource/connection.
type CreateConnectionResponse struct {
	Response *GenDBResponse    `json:"response,omitempty"`
	Spec     *ConnectionDetail `json:"spec,omitempty"`
}

func (c *Client) CreateConnection(name, connType string, connConfig map[string]string) (*CreateConnectionResponse, error) {
	reqBody := CreateConnectionRequest{
		Request: &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		Name:    name,
		Type:    connType,
		Config:  connConfig,
	}
	var resp CreateConnectionResponse
	if err := c.doJSON("POST", "/v1/datasource/connection", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) WaitForConnectionSync(connUUID string, timeoutSeconds int) (*GetConnectionResponse, error) {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(deadline) {
		resp, err := c.GetConnectionInfo(connUUID)
		if err != nil {
			return nil, err
		}
		if resp.Spec != nil {
			state := resp.Spec.SyncState
			if state == "SYNCED" || state == "SYNC_STATE_SYNCED" {
				return resp, nil
			}
			if state == "SYNC_STATE_FAILED" || state == "FAILED" {
				return resp, fmt.Errorf("sync failed for connection %s", connUUID)
			}
		}
		time.Sleep(5 * time.Second)
	}
	return nil, fmt.Errorf("sync timed out after %d seconds", timeoutSeconds)
}

// AddConnectionToProjectRequest holds the body for adding a connection to a project.
type AddConnectionToProjectRequest struct {
	Request        *GenDBRequest `json:"request,omitempty"`
	ConnectionUUID string        `json:"connection_uuid"`
}

func (c *Client) AddConnectionToProject(projectUUID, connUUID string) error {
	reqBody := AddConnectionToProjectRequest{
		Request:        &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		ConnectionUUID: connUUID,
	}
	var resp struct {
		Response *GenDBResponse `json:"response,omitempty"`
	}
	if err := c.doJSON("POST", "/v1/gendb/spec/"+projectUUID+"/datasource-connections", reqBody, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

// RemoveConnectionFromProjectRequest holds the body for removing a connection from a project.
type RemoveConnectionFromProjectRequest struct {
	Request        *GenDBRequest `json:"request,omitempty"`
	ConnectionUUID string        `json:"connection_uuid"`
}

func (c *Client) RemoveConnectionFromProject(projectUUID, connUUID string) error {
	reqBody := RemoveConnectionFromProjectRequest{
		Request:        &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		ConnectionUUID: connUUID,
	}
	var resp struct {
		Response *GenDBResponse `json:"response,omitempty"`
	}
	if err := c.doJSON("DELETE", "/v1/gendb/spec/"+projectUUID+"/datasource-connections", reqBody, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

// ListProjectConnectionsResponse holds the response for listing project connections.
type ListProjectConnectionsResponse struct {
	Response *GenDBResponse   `json:"response,omitempty"`
	Specs    []ConnectionSpec `json:"specs,omitempty"`
}

func (c *Client) ListProjectConnections(projectUUID string) (*ListProjectConnectionsResponse, error) {
	params := url.Values{}
	params.Set("project_uuid", projectUUID)
	var resp ListProjectConnectionsResponse
	if err := c.doJSON("GET", "/v1/connection?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Typed Connection Add (PagerDuty, FireHydrant, incident.io) ---

// PagerdutyConnectionInfo holds the auth credentials for a PagerDuty connection.
type PagerdutyConnectionInfo struct {
	ApiAccessKey string `json:"api_access_key,omitempty"`
}

// FirehydrantConnectionInfo holds the auth credentials for a FireHydrant connection.
type FirehydrantConnectionInfo struct {
	ApiKey string `json:"api_key,omitempty"`
}

// IncidentioConnectionInfo holds the auth credentials for an incident.io connection.
type IncidentioConnectionInfo struct {
	ApiKey string `json:"api_key,omitempty"`
}

// AddConnectionInput is the payload body for creating a connection.
type AddConnectionInput struct {
	Name                      string                     `json:"name,omitempty"`
	ConnectionType            string                     `json:"connection_type,omitempty"`
	PagerdutyConnectionInfo   *PagerdutyConnectionInfo   `json:"pagerduty_connection_info,omitempty"`
	FirehydrantConnectionInfo *FirehydrantConnectionInfo `json:"firehydrant_connection_info,omitempty"`
	IncidentioConnectionInfo  *IncidentioConnectionInfo  `json:"incidentio_connection_info,omitempty"`
}

// AddConnectionRequest wraps the connection input for POST /v1/connection.
type AddConnectionRequest struct {
	Connection AddConnectionInput `json:"connection"`
}

// ConnectionOperationResponse is the generic response envelope for connection mutations.
type ConnectionOperationResponse struct {
	RequestID    string `json:"request_id"`
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	UUID         string `json:"uuid"`
}

// AddConnectionResponse is returned by POST /v1/connection.
type AddConnectionResponse struct {
	Response ConnectionOperationResponse `json:"response"`
}

func (c *Client) AddConnection(req *AddConnectionRequest) (*AddConnectionResponse, error) {
	var resp AddConnectionResponse
	if err := c.doJSON("POST", "/v1/connection", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Resources ---

// ResourceID identifies a resource.
type ResourceID struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

// ResourceSpec describes a resource within a connection.
type ResourceSpec struct {
	ID             ResourceID `json:"id"`
	ConnectionUUID string     `json:"connection_uuid"`
	TelemetryType  string     `json:"telemetry_type"`
}

// ListResourcesResponse holds the list of resources.
type ListResourcesResponse struct {
	Specs []ResourceSpec `json:"specs"`
}

func (c *Client) ListConnectionResources(connectionUUID string, limit int) (*ListResourcesResponse, error) {
	params := url.Values{}
	params.Set("connection_uuid", connectionUUID)
	params.Set("pagination.limit", fmt.Sprintf("%d", limit))
	var resp ListResourcesResponse
	if err := c.doJSON("GET", "/v1/resource?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Discovery ---

// DiscoverResourcesResponse holds the response for resource discovery.
type DiscoverResourcesResponse struct {
	Response  *GenDBResponse `json:"response,omitempty"`
	Resources []ResourceSpec `json:"resources,omitempty"`
}

func (c *Client) DiscoverProjectResources(projectUUID, telemetryType, connectionType string) (*DiscoverResourcesResponse, error) {
	params := url.Values{}
	if telemetryType != "" {
		params.Set("telemetry_type", telemetryType)
	}
	if connectionType != "" {
		params.Set("connection_type", connectionType)
	}

	// First get project connections, then list resources for each
	connResp, err := c.ListProjectConnections(projectUUID)
	if err != nil {
		return nil, fmt.Errorf("listing project connections: %w", err)
	}

	var allResources []ResourceSpec
	for _, conn := range connResp.Specs {
		if connectionType != "" && conn.Type != connectionType {
			continue
		}
		resResp, err := c.ListConnectionResources(conn.UUID, 100)
		if err != nil {
			continue // skip connections with errors
		}
		for _, r := range resResp.Specs {
			if telemetryType != "" && r.TelemetryType != telemetryType {
				continue
			}
			allResources = append(allResources, r)
		}
	}

	return &DiscoverResourcesResponse{Resources: allResources}, nil
}

// --- Session Report ---

// SessionReportItem holds a single session report entry.
type SessionReportItem struct {
	CreateTime  string `json:"create_time,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	SessionLink string `json:"session_link,omitempty"`
	Summary     string `json:"summary,omitempty"`
	TimeSaved   int    `json:"time_saved,omitempty"`
}

func (c *Client) GetSessionReport(projectUUID string, sessionUUIDs []string) ([]SessionReportItem, error) {
	params := url.Values{}
	for _, s := range sessionUUIDs {
		params.Add("session_uuids", s)
	}
	params.Set("project_uuid", projectUUID)
	var resp []SessionReportItem
	if err := c.doJSON("GET", "/v1/inference/session_report?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// --- Investigation Enhancements ---

// CreateSessionFromAlertRequest holds the body for creating a session from an alert.
type CreateSessionFromAlertRequest struct {
	Request          *GenDBRequest `json:"request,omitempty"`
	OrganizationUUID string        `json:"organization_uuid,omitempty"`
	ProjectUUID      string        `json:"project_uuid,omitempty"`
	AlertID          string        `json:"alert_id,omitempty"`
}

func (c *Client) CreateSessionFromAlert(projectUUID, alertID string) (*NewSessionResponse, error) {
	reqBody := CreateSessionFromAlertRequest{
		Request:          &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		OrganizationUUID: c.orgUUID,
		ProjectUUID:      projectUUID,
		AlertID:          alertID,
	}
	var resp NewSessionResponse
	if err := c.doJSON("POST", "/v1/inference/session:create", reqBody, &resp); err != nil {
		return nil, err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return nil, fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return &resp, nil
}

// QueryExecution describes a query that was executed during an investigation.
type QueryExecution struct {
	ID            string `json:"id"`
	Query         string `json:"query"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	ExecutionTime string `json:"execution_time,omitempty"`
	ResultCount   int    `json:"result_count,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// GetInvestigationQueriesResponse holds the response for queries.
type GetInvestigationQueriesResponse struct {
	Response *GenDBResponse   `json:"response,omitempty"`
	Queries  []QueryExecution `json:"queries,omitempty"`
}

func (c *Client) GetInvestigationQueries(projectUUID, sessionUUID string) (*GetInvestigationQueriesResponse, error) {
	// Queries are extracted from chain_of_thoughts in the inspect response.
	inspectResp, err := c.SessionInspect(projectUUID, sessionUUID)
	if err != nil {
		return nil, err
	}
	var queries []QueryExecution
	for _, pc := range inspectResp.PromptCycle {
		for _, cot := range pc.ChainOfThoughts {
			q := QueryExecution{
				ID:     cot.ID,
				Query:  cot.Description,
				Status: cot.Status,
			}
			if len(cot.Sources) > 0 {
				q.Source = cot.Sources[0]
			}
			queries = append(queries, q)
		}
	}
	return &GetInvestigationQueriesResponse{Queries: queries}, nil
}

// --- Instructions ---

// InstructionSpec describes a project instruction.
type InstructionSpec struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	Enabled    bool   `json:"enabled"`
	CreateTime string `json:"create_time,omitempty"`
	UpdateTime string `json:"update_time,omitempty"`
}

// ListInstructionsResponse holds the response from GET /v1/instruction.
type ListInstructionsResponse struct {
	Response     *GenDBResponse    `json:"response,omitempty"`
	Instructions []InstructionSpec `json:"instructions,omitempty"`
}

func (c *Client) ListInstructions(projectUUID string) (*ListInstructionsResponse, error) {
	params := url.Values{}
	params.Set("project_uuid", projectUUID)
	var resp ListInstructionsResponse
	if err := c.doJSON("GET", "/v1/instruction?"+params.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateInstructionRequest holds the body for POST /v1/instruction.
type CreateInstructionRequest struct {
	Instruction struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"instruction"`
	ProjectUUID      string `json:"project_uuid,omitempty"`
	OrganizationUUID string `json:"organization_uuid,omitempty"`
}

// CreateInstructionResponse holds the response from creating an instruction.
type CreateInstructionResponse struct {
	Response    *GenDBResponse   `json:"response,omitempty"`
	Instruction *InstructionSpec `json:"instruction,omitempty"`
}

func (c *Client) CreateInstruction(projectUUID, name, instrType, content string) (*CreateInstructionResponse, error) {
	reqBody := CreateInstructionRequest{
		ProjectUUID:      projectUUID,
		OrganizationUUID: c.orgUUID,
	}
	reqBody.Instruction.Name = name
	reqBody.Instruction.Type = instrType
	reqBody.Instruction.Content = content
	var resp CreateInstructionResponse
	if err := c.doJSON("POST", "/v1/instruction", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateInstructionStatusRequest holds the body for PUT /v1/instruction/{uuid}.
type UpdateInstructionStatusRequest struct {
	Status string `json:"status"`
}

func (c *Client) UpdateInstructionStatus(instrUUID string, enabled bool) error {
	status := "INSTRUCTION_STATUS_DISABLED"
	if enabled {
		status = "INSTRUCTION_STATUS_ENABLED"
	}
	reqBody := UpdateInstructionStatusRequest{Status: status}
	var resp struct {
		Response *GenDBResponse `json:"response,omitempty"`
	}
	if err := c.doJSON("PUT", "/v1/instruction/"+instrUUID, reqBody, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

func (c *Client) DeleteInstruction(instrUUID string) error {
	var resp struct {
		Response *GenDBResponse `json:"response,omitempty"`
	}
	if err := c.doJSON("DELETE", "/v1/instruction/"+instrUUID, nil, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

// ValidateInstructionRequest holds the body for POST /v1/instruction/validate.
type ValidateInstructionRequest struct {
	Instruction struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"instruction"`
}

// ValidateInstructionResponse holds validation results.
type ValidateInstructionResponse struct {
	Response    *GenDBResponse   `json:"response,omitempty"`
	Instruction *InstructionSpec `json:"instruction,omitempty"`
}

func (c *Client) ValidateInstruction(instrType, content string) (*ValidateInstructionResponse, error) {
	var reqBody ValidateInstructionRequest
	reqBody.Instruction.Type = instrType
	reqBody.Instruction.Content = content
	var resp ValidateInstructionResponse
	if err := c.doJSON("POST", "/v1/instruction/validate", reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ApplySessionInstructionRequest holds the body for POST /v1/inference/session/{uuid}/instructions.
type ApplySessionInstructionRequest struct {
	Request *GenDBRequest `json:"request,omitempty"`
	Type    string        `json:"type"`
	Content string        `json:"content"`
}

func (c *Client) ApplySessionInstruction(sessionUUID, instrType, content string) error {
	reqBody := ApplySessionInstructionRequest{
		Request: &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
		Type:    instrType,
		Content: content,
	}
	var resp struct {
		Response *GenDBResponse `json:"response,omitempty"`
	}
	if err := c.doJSON("POST", "/v1/inference/session/"+sessionUUID+"/instructions", reqBody, &resp); err != nil {
		return err
	}
	if resp.Response != nil && resp.Response.ErrorCode != 0 {
		return fmt.Errorf("server error: %s", resp.Response.ErrorMessage)
	}
	return nil
}

// RerunSessionResponse holds the response from POST /v1/inference/session/{uuid}:rerun.
type RerunSessionResponse struct {
	Response    *GenDBResponse `json:"response,omitempty"`
	SessionUUID string         `json:"session_uuid,omitempty"`
}

func (c *Client) RerunSession(sessionUUID string) (*RerunSessionResponse, error) {
	reqBody := struct {
		Request *GenDBRequest `json:"request,omitempty"`
	}{
		Request: &GenDBRequest{ClientIdentifier: "hawkeye-cli", UUID: c.orgUUID},
	}
	var resp RerunSessionResponse
	if err := c.doJSON("POST", "/v1/inference/session/"+sessionUUID+":rerun", reqBody, &resp); err != nil {
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
