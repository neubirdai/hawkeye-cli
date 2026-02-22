package api

// HawkeyeAPI defines the interface for the Hawkeye API client.
// *Client satisfies this interface. TUI and tests can use mock implementations.
type HawkeyeAPI interface {
	Login(email, password string) (*LoginResponse, error)
	FetchUserInfo() (*UserSpec, error)
	NewSession(projectUUID string) (*NewSessionResponse, error)
	SessionList(projectUUID string, limit int, filters []PaginationFilter) (*SessionListResponse, error)
	SessionInspect(projectUUID, sessionUUID string) (*SessionInspectResponse, error)
	GetSessionSummary(projectUUID, sessionUUID string) (*GetSessionSummaryResponse, error)
	ProcessPromptStream(projectUUID, sessionUUID, prompt string, cb StreamCallback) error
	PutRating(projectUUID, sessionUUID string, itemIDs []RatingItemID, rating, reason string) error
	PromptLibrary(projectUUID string) (*PromptLibraryResponse, error)
	ListProjects() (*ListProjectResponse, error)
	GetProject(projectUUID string) (*GetProjectResponse, error)
	CreateProject(name, description string) (*CreateProjectResponse, error)
	UpdateProject(projectUUID, name, description string) (*UpdateProjectResponse, error)
	DeleteProject(projectUUID string) error
	GetIncidentReport() (*IncidentReportResponse, error)
	ListConnections(projectUUID string) (*ListConnectionsResponse, error)
	ListConnectionResources(connUUID string, limit int) (*ListResourcesResponse, error)
	GetConnectionInfo(connUUID string) (*GetConnectionResponse, error)
	CreateConnection(name, connType string, connConfig map[string]string) (*CreateConnectionResponse, error)
	WaitForConnectionSync(connUUID string, timeoutSeconds int) (*GetConnectionResponse, error)
	AddConnectionToProject(projectUUID, connUUID string) error
	RemoveConnectionFromProject(projectUUID, connUUID string) error
	ListProjectConnections(projectUUID string) (*ListProjectConnectionsResponse, error)
	AddConnection(req *AddConnectionRequest) (*AddConnectionResponse, error)
	ListInstructions(projectUUID string) (*ListInstructionsResponse, error)
	CreateInstruction(projectUUID, name, instrType, content string) (*CreateInstructionResponse, error)
	UpdateInstructionStatus(instrUUID string, enabled bool) error
	DeleteInstruction(instrUUID string) error
	ValidateInstruction(instrType, content string) (*ValidateInstructionResponse, error)
	ApplySessionInstruction(sessionUUID, instrType, content string) error
	RerunSession(sessionUUID string) (*RerunSessionResponse, error)
	CreateSessionFromAlert(projectUUID, alertID string) (*NewSessionResponse, error)
	GetInvestigationQueries(projectUUID, sessionUUID string) (*GetInvestigationQueriesResponse, error)
	DiscoverProjectResources(projectUUID, telemetryType, connectionType string) (*DiscoverResourcesResponse, error)
	GetSessionReport(projectUUID string, sessionUUIDs []string) ([]SessionReportItem, error)
}
