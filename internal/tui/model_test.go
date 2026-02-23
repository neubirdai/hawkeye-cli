package tui

import (
	"fmt"
	"testing"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/service"
)

// mockAPI implements api.HawkeyeAPI for testing.
type mockAPI struct {
	sessions    []api.SessionInfo
	projects    []api.ProjectSpec
	prompts     []api.InitialPrompt
	summary     *api.GetSessionSummaryResponse
	inspect     *api.SessionInspectResponse
	report      *api.IncidentReportResponse
	connections *api.ListConnectionsResponse
	resources   *api.ListResourcesResponse

	err error // if set, all methods return this error
}

func (m *mockAPI) Login(email, password string) (*api.LoginResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.LoginResponse{AccessToken: "test-token"}, nil
}

func (m *mockAPI) FetchUserInfo() (*api.UserSpec, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.UserSpec{UUID: "user-1", OrgUUID: "org-1"}, nil
}

func (m *mockAPI) NewSession(projectUUID string) (*api.NewSessionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.NewSessionResponse{SessionUUID: "new-session-uuid"}, nil
}

func (m *mockAPI) SessionList(projectUUID string, limit int, filters []api.PaginationFilter) (*api.SessionListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.SessionListResponse{Sessions: m.sessions}, nil
}

func (m *mockAPI) SessionInspect(projectUUID, sessionUUID string) (*api.SessionInspectResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.inspect != nil {
		return m.inspect, nil
	}
	return &api.SessionInspectResponse{}, nil
}

func (m *mockAPI) GetSessionSummary(projectUUID, sessionUUID string) (*api.GetSessionSummaryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.summary != nil {
		return m.summary, nil
	}
	return &api.GetSessionSummaryResponse{}, nil
}

func (m *mockAPI) ProcessPromptStream(projectUUID, sessionUUID, prompt string, cb api.StreamCallback) error {
	if m.err != nil {
		return m.err
	}
	// Send a minimal end_turn response
	cb(&api.ProcessPromptResponse{
		Message: &api.Message{
			Content: &api.Content{
				ContentType: "CONTENT_TYPE_CHAT_RESPONSE",
				Parts:       []string{"test response"},
			},
			EndTurn: true,
		},
	})
	return nil
}

func (m *mockAPI) PutRating(projectUUID, sessionUUID string, itemIDs []api.RatingItemID, rating, reason string) error {
	return m.err
}

func (m *mockAPI) PromptLibrary(projectUUID string) (*api.PromptLibraryResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.PromptLibraryResponse{Items: m.prompts}, nil
}

func (m *mockAPI) ListProjects() (*api.ListProjectResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.ListProjectResponse{Specs: m.projects}, nil
}

func (m *mockAPI) GetProject(projectUUID string) (*api.GetProjectResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.GetProjectResponse{Spec: &api.ProjectDetail{UUID: projectUUID, Name: "Test Project"}}, nil
}

func (m *mockAPI) CreateProject(name, description string) (*api.CreateProjectResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.CreateProjectResponse{Spec: &api.ProjectDetail{UUID: "new-proj-uuid", Name: name, Description: description}}, nil
}

func (m *mockAPI) UpdateProject(projectUUID, name, description string) (*api.UpdateProjectResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.UpdateProjectResponse{Spec: &api.ProjectDetail{UUID: projectUUID, Name: name}}, nil
}

func (m *mockAPI) DeleteProject(projectUUID string) error {
	return m.err
}

func (m *mockAPI) GetIncidentReport() (*api.IncidentReportResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.report != nil {
		return m.report, nil
	}
	return &api.IncidentReportResponse{}, nil
}

func (m *mockAPI) ListConnections(projectUUID string) (*api.ListConnectionsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.connections != nil {
		return m.connections, nil
	}
	return &api.ListConnectionsResponse{}, nil
}

func (m *mockAPI) ListConnectionResources(connUUID string, limit int) (*api.ListResourcesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.resources != nil {
		return m.resources, nil
	}
	return &api.ListResourcesResponse{}, nil
}

func (m *mockAPI) GetConnectionInfo(connUUID string) (*api.GetConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.GetConnectionResponse{Spec: &api.ConnectionDetail{UUID: connUUID, Name: "Test Connection", Type: "datadog"}}, nil
}

func (m *mockAPI) CreateConnection(name, connType string, connConfig map[string]string) (*api.CreateConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.CreateConnectionResponse{Spec: &api.ConnectionDetail{UUID: "new-conn-uuid", Name: name, Type: connType}}, nil
}

func (m *mockAPI) WaitForConnectionSync(connUUID string, timeoutSeconds int) (*api.GetConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.GetConnectionResponse{Spec: &api.ConnectionDetail{UUID: connUUID, SyncState: "SYNCED"}}, nil
}

func (m *mockAPI) AddConnectionToProject(projectUUID, connUUID string) error {
	return m.err
}

func (m *mockAPI) RemoveConnectionFromProject(projectUUID, connUUID string) error {
	return m.err
}

func (m *mockAPI) ListProjectConnections(projectUUID string) (*api.ListProjectConnectionsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.ListProjectConnectionsResponse{}, nil
}

func (m *mockAPI) AddConnection(req *api.AddConnectionRequest) (*api.AddConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.AddConnectionResponse{}, nil
}

func (m *mockAPI) ListInstructions(projectUUID string) (*api.ListInstructionsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.ListInstructionsResponse{}, nil
}

func (m *mockAPI) CreateInstruction(projectUUID, name, instrType, content string) (*api.CreateInstructionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.CreateInstructionResponse{Instruction: &api.InstructionSpec{UUID: "new-instr-uuid", Name: name}}, nil
}

func (m *mockAPI) UpdateInstructionStatus(instrUUID string, enabled bool) error {
	return m.err
}

func (m *mockAPI) DeleteInstruction(instrUUID string) error {
	return m.err
}

func (m *mockAPI) ValidateInstruction(instrType, content string) (*api.ValidateInstructionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.ValidateInstructionResponse{Instruction: &api.InstructionSpec{Name: "validated"}}, nil
}

func (m *mockAPI) ApplySessionInstruction(sessionUUID, instrType, content string) error {
	return m.err
}

func (m *mockAPI) RerunSession(sessionUUID string) (*api.RerunSessionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.RerunSessionResponse{SessionUUID: sessionUUID}, nil
}

func (m *mockAPI) CreateSessionFromAlert(projectUUID, alertID string) (*api.NewSessionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.NewSessionResponse{SessionUUID: "alert-session-uuid"}, nil
}

func (m *mockAPI) GetInvestigationQueries(projectUUID, sessionUUID string) (*api.GetInvestigationQueriesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.GetInvestigationQueriesResponse{}, nil
}

func (m *mockAPI) DiscoverProjectResources(projectUUID, telemetryType, connectionType string) (*api.DiscoverResourcesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.DiscoverResourcesResponse{}, nil
}

func (m *mockAPI) GetSessionReport(projectUUID string, sessionUUIDs []string) ([]api.SessionReportItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, nil
}

// Verify mockAPI satisfies the interface at compile time.
var _ api.HawkeyeAPI = (*mockAPI)(nil)

func newTestModel() model {
	m := initialModel("test", "")
	m.cfg = &config.Config{
		Server:    "http://localhost:8080",
		Token:     "test-token",
		OrgUUID:   "org-1",
		ProjectID: "proj-1",
	}
	m.client = &mockAPI{}
	m.ready = true
	m.width = 80
	m.height = 24
	return m
}

func TestDispatchCommand(t *testing.T) {
	tests := []struct {
		input     string
		wantMode  appMode
		wantError bool
	}{
		{"/help", modeIdle, false},
		{"/config", modeIdle, false},
		{"/clear", modeIdle, false},
		{"/quit", modeIdle, false}, // quit returns tea.Quit cmd
		{"/unknown", modeIdle, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := newTestModel()
			result, _ := m.dispatchCommand(tt.input)
			rm := result.(model)
			if rm.mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", rm.mode, tt.wantMode)
			}
		})
	}
}

func TestDispatchInput(t *testing.T) {
	t.Run("question mark shows help", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.dispatchInput("?")
		rm := result.(model)
		if rm.mode != modeIdle {
			t.Errorf("mode = %d, want modeIdle", rm.mode)
		}
	})

	t.Run("slash dispatches command", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.dispatchInput("/config")
		rm := result.(model)
		if rm.mode != modeIdle {
			t.Errorf("mode = %d, want modeIdle", rm.mode)
		}
	})

	t.Run("plain text starts investigation", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.dispatchInput("Why is the API failing?")
		rm := result.(model)
		if rm.mode != modeStreaming {
			t.Errorf("mode = %d, want modeStreaming", rm.mode)
		}
	})

	t.Run("investigation without client shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		result, cmd := m.dispatchInput("test question")
		rm := result.(model)
		if rm.mode != modeIdle {
			t.Errorf("mode = %d, want modeIdle", rm.mode)
		}
		if cmd == nil {
			t.Error("expected error message cmd, got nil")
		}
	})
}

func TestLoginFlow(t *testing.T) {
	t.Run("login without args enters URL mode", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.cmdLogin(nil)
		rm := result.(model)
		if rm.mode != modeLoginURL {
			t.Errorf("mode = %d, want modeLoginURL", rm.mode)
		}
	})

	t.Run("login with URL enters user mode", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.cmdLogin([]string{"https://test.example.com"})
		rm := result.(model)
		if rm.mode != modeLoginUser {
			t.Errorf("mode = %d, want modeLoginUser", rm.mode)
		}
		if rm.loginURL != "https://test.example.com" {
			t.Errorf("loginURL = %q, want %q", rm.loginURL, "https://test.example.com")
		}
	})

	t.Run("URL submit transitions to user mode", func(t *testing.T) {
		m := newTestModel()
		m.mode = modeLoginURL
		result, _ := m.handleLoginURLSubmit("https://server.com")
		rm := result.(model)
		if rm.mode != modeLoginUser {
			t.Errorf("mode = %d, want modeLoginUser", rm.mode)
		}
		if rm.loginURL != "https://server.com" {
			t.Errorf("loginURL = %q", rm.loginURL)
		}
	})

	t.Run("user submit transitions to pass mode", func(t *testing.T) {
		m := newTestModel()
		m.mode = modeLoginUser
		result, _ := m.handleLoginUserSubmit("user@test.com")
		rm := result.(model)
		if rm.mode != modeLoginPass {
			t.Errorf("mode = %d, want modeLoginPass", rm.mode)
		}
		if rm.loginUser != "user@test.com" {
			t.Errorf("loginUser = %q", rm.loginUser)
		}
	})
}

func TestSessionCommands(t *testing.T) {
	t.Run("session with uuid sets sessionID", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.cmdSetSession([]string{"new-session-id"})
		rm := result.(model)
		if rm.sessionID != "new-session-id" {
			t.Errorf("sessionID = %q, want %q", rm.sessionID, "new-session-id")
		}
	})

	t.Run("session without args and no client shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdSetSession(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("session without args and no project shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg.ProjectID = ""
		_, cmd := m.cmdSetSession(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("session loaded enters select mode", func(t *testing.T) {
		m := newTestModel()
		sessions := []api.SessionInfo{
			{SessionUUID: "s1", Name: "First"},
			{SessionUUID: "s2", Name: "Second"},
		}
		result, _ := m.handleSessionsLoaded(sessionsLoadedMsg{sessions: sessions})
		rm := result.(model)
		if rm.mode != modeSessionSelect {
			t.Errorf("mode = %v, want modeSessionSelect", rm.mode)
		}
		if len(rm.sessionList) != 2 {
			t.Errorf("sessionList len = %d, want 2", len(rm.sessionList))
		}
	})

	t.Run("session loaded empty shows warning", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleSessionsLoaded(sessionsLoadedMsg{})
		if cmd == nil {
			t.Error("expected warning cmd, got nil")
		}
	})
}

func TestCommandRequiresAuth(t *testing.T) {
	commands := []struct {
		name string
		fn   func(m model) (model, bool)
	}{
		{"inspect", func(m model) (model, bool) {
			r, c := m.cmdInspect([]string{"uuid"})
			return r.(model), c != nil
		}},
		{"summary", func(m model) (model, bool) {
			r, c := m.cmdSummary([]string{"uuid"})
			return r.(model), c != nil
		}},
		{"feedback", func(m model) (model, bool) {
			r, c := m.cmdFeedback([]string{"uuid"})
			return r.(model), c != nil
		}},
		{"prompts", func(m model) (model, bool) {
			r, c := m.cmdPrompts()
			return r.(model), c != nil
		}},
		{"projects", func(m model) (model, bool) {
			r, c := m.cmdProjects(nil)
			return r.(model), c != nil
		}},
		{"score", func(m model) (model, bool) {
			r, c := m.cmdScore([]string{"uuid"})
			return r.(model), c != nil
		}},
		{"report", func(m model) (model, bool) {
			r, c := m.cmdReport()
			return r.(model), c != nil
		}},
		{"connections", func(m model) (model, bool) {
			r, c := m.cmdConnections(nil)
			return r.(model), c != nil
		}},
	}

	for _, tc := range commands {
		t.Run(tc.name+" without auth", func(t *testing.T) {
			m := newTestModel()
			m.client = nil
			_, hasCmd := tc.fn(m)
			if !hasCmd {
				t.Error("expected error cmd when not logged in")
			}
		})
	}
}

func TestLinkCommand(t *testing.T) {
	t.Run("link with session UUID", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdLink([]string{"sess-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("link falls back to active session", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "active-sess"
		_, cmd := m.cmdLink(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("link without session shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdLink(nil)
		if cmd == nil {
			t.Error("expected usage message cmd, got nil")
		}
	})

	t.Run("link without config shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg = nil
		_, cmd := m.cmdLink([]string{"sess"})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})
}

func TestScoreFallback(t *testing.T) {
	t.Run("falls back to active session", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "active-sess"
		_, cmd := m.cmdScore(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("without session shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdScore(nil)
		if cmd == nil {
			t.Error("expected usage message, got nil")
		}
	})
}

func TestHandleLoginResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		m.mode = modeLoginPass
		cfg := &config.Config{
			Server:   "http://test.com",
			Username: "user",
			Token:    "token",
			OrgUUID:  "org",
		}
		result, _ := m.handleLoginResult(loginResultMsg{cfg: cfg})
		rm := result.(model)
		if rm.mode != modeIdle {
			t.Errorf("mode = %d, want modeIdle", rm.mode)
		}
		if rm.cfg != cfg {
			t.Error("config not set")
		}
	})

	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		m.mode = modeLoginPass
		result, _ := m.handleLoginResult(loginResultMsg{err: fmt.Errorf("auth failed")})
		rm := result.(model)
		if rm.mode != modeIdle {
			t.Errorf("mode = %d, want modeIdle", rm.mode)
		}
	})
}

// ─── Phase 1: Project CRUD ──────────────────────────────────────────────────

func TestProjectInfoCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectInfo(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectInfo([]string{"proj-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestProjectCreateCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectCreate(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with name returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectCreate([]string{"My", "Project"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestProjectDeleteCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectDelete(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdProjectDelete([]string{"proj-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleProjectInfo(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectInfo(projectInfoMsg{err: fmt.Errorf("not found")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		detail := service.ProjectDetailDisplay{
			UUID: "proj-1", Name: "Test", Description: "desc", Ready: true,
		}
		_, cmd := m.handleProjectInfo(projectInfoMsg{detail: detail})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

func TestHandleProjectCreate(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectCreate(projectCreateMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success with spec", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectCreate(projectCreateMsg{spec: &api.ProjectDetail{UUID: "new-uuid", Name: "Test"}})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})

	t.Run("success without spec", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectCreate(projectCreateMsg{})
		if cmd == nil {
			t.Error("expected fallback cmd, got nil")
		}
	})
}

func TestHandleProjectDelete(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectDelete(projectDeleteMsg{uuid: "proj-1", err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleProjectDelete(projectDeleteMsg{uuid: "proj-1"})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

func TestProjectSubcommandDispatch(t *testing.T) {
	m := newTestModel()

	tests := []struct {
		args    []string
		wantCmd bool
	}{
		{[]string{"info", "uuid"}, true},
		{[]string{"create", "name"}, true},
		{[]string{"delete", "uuid"}, true},
		{nil, true}, // list
	}

	for _, tt := range tests {
		name := "list"
		if len(tt.args) > 0 {
			name = tt.args[0]
		}
		t.Run(name, func(t *testing.T) {
			_, cmd := m.cmdProjects(tt.args)
			if tt.wantCmd && cmd == nil {
				t.Error("expected cmd, got nil")
			}
		})
	}
}

// ─── Phase 2: Connection Commands ───────────────────────────────────────────

func TestConnectionTypesCommand(t *testing.T) {
	m := newTestModel()
	_, cmd := m.cmdConnectionTypes()
	if cmd == nil {
		t.Error("expected cmd, got nil")
	}
}

func TestConnectionInfoCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionInfo(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionInfo([]string{"conn-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestConnectionAddCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionAdd(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionAdd([]string{"conn-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestConnectionRemoveCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionRemove(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdConnectionRemove([]string{"conn-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleConnInfo(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleConnInfo(connInfoMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		detail := service.ConnectionDetailDisplay{
			UUID: "c1", Name: "DD", Type: "datadog", SyncState: "SYNCED",
		}
		_, cmd := m.handleConnInfo(connInfoMsg{detail: detail})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

func TestHandleConnAdd(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleConnAdd(connAddMsg{connUUID: "c1", err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleConnAdd(connAddMsg{connUUID: "c1"})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

func TestHandleConnRemove(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleConnRemove(connRemoveMsg{connUUID: "c1", err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleConnRemove(connRemoveMsg{connUUID: "c1"})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

func TestConnectionSubcommandDispatch(t *testing.T) {
	m := newTestModel()

	tests := []struct {
		args    []string
		wantCmd bool
	}{
		{[]string{"types"}, true},
		{[]string{"info", "uuid"}, true},
		{[]string{"add", "uuid"}, true},
		{[]string{"remove", "uuid"}, true},
		{nil, true}, // list
	}

	for _, tt := range tests {
		name := "list"
		if len(tt.args) > 0 {
			name = tt.args[0]
		}
		t.Run(name, func(t *testing.T) {
			_, cmd := m.cmdConnections(tt.args)
			if tt.wantCmd && cmd == nil {
				t.Error("expected cmd, got nil")
			}
		})
	}
}

// ─── Phase 3: Instructions ──────────────────────────────────────────────────

func TestInstructionsCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdInstructions(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("without project shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg.ProjectID = ""
		_, cmd := m.cmdInstructions(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("list returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructions(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("subcommand dispatch", func(t *testing.T) {
		m := newTestModel()
		for _, sub := range []string{"create", "enable", "disable", "delete"} {
			_, cmd := m.cmdInstructions([]string{sub, "arg"})
			if cmd == nil {
				t.Errorf("expected cmd for subcommand %q, got nil", sub)
			}
		}
	})
}

func TestInstructionCreateCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionCreate(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with name returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionCreate([]string{"My", "Rule"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestInstructionToggleCommand(t *testing.T) {
	t.Run("enable no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionToggle(nil, true)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("disable no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionToggle(nil, false)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("enable with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionToggle([]string{"instr-uuid"}, true)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestInstructionDeleteCommand(t *testing.T) {
	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionDelete(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInstructionDelete([]string{"instr-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleInstructionsLoaded(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionsLoaded(instructionsLoadedMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionsLoaded(instructionsLoadedMsg{instructions: nil})
		if cmd == nil {
			t.Error("expected empty message cmd, got nil")
		}
	})

	t.Run("with instructions", func(t *testing.T) {
		m := newTestModel()
		instrs := []service.InstructionDisplay{
			{UUID: "i1", Name: "Rule 1", Type: "filter", Enabled: true},
			{UUID: "i2", Name: "Rule 2", Type: "system", Enabled: false},
		}
		_, cmd := m.handleInstructionsLoaded(instructionsLoadedMsg{instructions: instrs})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

func TestHandleInstructionCreate(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionCreate(instructionCreateMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success with spec", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionCreate(instructionCreateMsg{spec: &api.InstructionSpec{UUID: "i1", Name: "Rule 1"}})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

func TestHandleInstructionToggle(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionToggle(instructionToggleMsg{uuid: "i1", err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("enabled", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionToggle(instructionToggleMsg{uuid: "i1", enabled: true})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionToggle(instructionToggleMsg{uuid: "i1", enabled: false})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

func TestHandleInstructionDelete(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionDelete(instructionDeleteMsg{uuid: "i1", err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleInstructionDelete(instructionDeleteMsg{uuid: "i1"})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

// ─── Phase 3 cont: Rerun ───────────────────────────────────────────────────

func TestRerunCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdRerun(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("no args no session shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdRerun(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("falls back to active session", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "active-sess"
		_, cmd := m.cmdRerun(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("with explicit uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdRerun([]string{"sess-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleRerunResult(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleRerunResult(rerunResultMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleRerunResult(rerunResultMsg{sessionUUID: "sess-1"})
		if cmd == nil {
			t.Error("expected success cmd, got nil")
		}
	})
}

// ─── Phase 4: Investigation Enhancements ────────────────────────────────────

func TestInvestigateAlertCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdInvestigateAlert(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("without project shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg.ProjectID = ""
		_, cmd := m.cmdInvestigateAlert([]string{"alert-1"})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("no args shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdInvestigateAlert(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("with alert-id enters streaming mode", func(t *testing.T) {
		m := newTestModel()
		result, cmd := m.cmdInvestigateAlert([]string{"alert-42"})
		rm := result.(model)
		if rm.mode != modeStreaming {
			t.Errorf("mode = %d, want modeStreaming", rm.mode)
		}
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestQueriesCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdQueries(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("no args no session shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdQueries(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("falls back to active session", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "active-sess"
		_, cmd := m.cmdQueries(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("with explicit uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdQueries([]string{"sess-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleQueriesResult(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleQueriesResult(queriesResultMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("empty", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleQueriesResult(queriesResultMsg{queries: nil})
		if cmd == nil {
			t.Error("expected empty message cmd, got nil")
		}
	})

	t.Run("with queries", func(t *testing.T) {
		m := newTestModel()
		queries := []service.QueryDisplay{
			{Query: "SELECT 1", Source: "postgres", Status: "SUCCESS"},
			{Query: "metric:cpu", Source: "datadog", Status: "FAILED"},
		}
		_, cmd := m.handleQueriesResult(queriesResultMsg{queries: queries})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

// ─── Phase 5: Discovery & Reports ───────────────────────────────────────────

func TestDiscoverCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdDiscover()
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("without project shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg.ProjectID = ""
		_, cmd := m.cmdDiscover()
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("with config returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdDiscover()
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleDiscoverResult(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleDiscoverResult(discoverResultMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("empty", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleDiscoverResult(discoverResultMsg{resources: nil})
		if cmd == nil {
			t.Error("expected empty message cmd, got nil")
		}
	})

	t.Run("with resources", func(t *testing.T) {
		m := newTestModel()
		resources := []service.DiscoveredResource{
			{Name: "cpu-usage", TelemetryType: "metric", ConnectionUUID: "c1"},
			{Name: "error-logs", TelemetryType: "log", ConnectionUUID: "c2"},
		}
		_, cmd := m.handleDiscoverResult(discoverResultMsg{resources: resources})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

func TestSessionReportCommand(t *testing.T) {
	t.Run("without auth shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdSessionReport(nil)
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("no args no session shows usage", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdSessionReport(nil)
		if cmd == nil {
			t.Error("expected usage cmd, got nil")
		}
	})

	t.Run("falls back to active session", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "active-sess"
		_, cmd := m.cmdSessionReport(nil)
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})

	t.Run("with explicit uuid returns cmd", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.cmdSessionReport([]string{"sess-uuid"})
		if cmd == nil {
			t.Error("expected cmd, got nil")
		}
	})
}

func TestHandleSessionReport(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleSessionReport(sessionReportMsg{err: fmt.Errorf("fail")})
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("with report items", func(t *testing.T) {
		m := newTestModel()
		items := []api.SessionReportItem{
			{Summary: "Root cause found", TimeSaved: 1500},
		}
		_, cmd := m.handleSessionReport(sessionReportMsg{items: items})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})

	t.Run("empty report", func(t *testing.T) {
		m := newTestModel()
		_, cmd := m.handleSessionReport(sessionReportMsg{})
		if cmd == nil {
			t.Error("expected output cmd, got nil")
		}
	})
}

// ─── TUI Dispatch integration ───────────────────────────────────────────────

func TestNewSlashCommandDispatch(t *testing.T) {
	tests := []struct {
		input   string
		wantNil bool
	}{
		{"/instructions", false},
		{"/instructions create test", false},
		{"/instructions enable uuid", false},
		{"/instructions disable uuid", false},
		{"/instructions delete uuid", false},
		{"/rerun sess-uuid", false},
		{"/investigate-alert alert-1", false},
		{"/queries sess-uuid", false},
		{"/discover", false},
		{"/session-report sess-uuid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := newTestModel()
			_, cmd := m.dispatchCommand(tt.input)
			if !tt.wantNil && cmd == nil {
				t.Error("expected cmd, got nil")
			}
		})
	}
}

func TestResetStreamState(t *testing.T) {
	m := newTestModel()
	// Dirty the processor so we can confirm it gets replaced
	m.processor.Process(streamChunkMsg{
		contentType: "CONTENT_TYPE_PROGRESS_STATUS",
		text:        "(Loading Investigation Programs)",
	})
	if m.processor.LastStatus() == "" {
		t.Fatal("expected non-empty status before reset")
	}

	m.resetStreamState()

	if m.processor == nil {
		t.Fatal("processor should not be nil after reset")
	}
	if m.processor.LastStatus() != "" {
		t.Errorf("LastStatus() = %q, want empty after reset", m.processor.LastStatus())
	}
	if m.streamPrompt != "" {
		t.Errorf("streamPrompt = %q, want empty after reset", m.streamPrompt)
	}
}
