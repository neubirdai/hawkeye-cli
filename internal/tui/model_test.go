package tui

import (
	"fmt"
	"testing"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
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

func (m *mockAPI) AddConnection(req *api.AddConnectionRequest) (*api.AddConnectionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &api.AddConnectionResponse{}, nil
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
	t.Run("sessions without client shows error", func(t *testing.T) {
		m := newTestModel()
		m.client = nil
		_, cmd := m.cmdSessions()
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("sessions without project shows error", func(t *testing.T) {
		m := newTestModel()
		m.cfg.ProjectID = ""
		_, cmd := m.cmdSessions()
		if cmd == nil {
			t.Error("expected error cmd, got nil")
		}
	})

	t.Run("set session updates sessionID", func(t *testing.T) {
		m := newTestModel()
		result, _ := m.cmdSetSession([]string{"new-session-id"})
		rm := result.(model)
		if rm.sessionID != "new-session-id" {
			t.Errorf("sessionID = %q, want %q", rm.sessionID, "new-session-id")
		}
	})

	t.Run("set session without args shows current", func(t *testing.T) {
		m := newTestModel()
		m.sessionID = "existing-id"
		_, cmd := m.cmdSetSession(nil)
		if cmd == nil {
			t.Error("expected info cmd, got nil")
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
			r, c := m.cmdProjects()
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

func TestResetStreamState(t *testing.T) {
	m := newTestModel()
	m.chatPrinted = 100
	m.chatBuffer = "partial"
	m.cotStepActive = true
	m.chatStreaming = true
	m.lastStatus = "working..."
	m.cotStepNum = 3

	m.resetStreamState()

	if m.chatPrinted != 0 {
		t.Errorf("chatPrinted = %d, want 0", m.chatPrinted)
	}
	if m.chatBuffer != "" {
		t.Errorf("chatBuffer = %q, want empty", m.chatBuffer)
	}
	if m.cotStepActive {
		t.Error("cotStepActive should be false")
	}
	if m.chatStreaming {
		t.Error("chatStreaming should be false")
	}
	if m.lastStatus != "" {
		t.Errorf("lastStatus = %q, want empty", m.lastStatus)
	}
	if m.cotStepNum != 0 {
		t.Errorf("cotStepNum = %d, want 0", m.cotStepNum)
	}
}
